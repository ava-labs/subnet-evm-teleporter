// (c) 2019-2020, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package precompile

import (
	"errors"
	"fmt"
	"math/big"
	"os"

	"github.com/ava-labs/subnet-evm/vmerrs"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

// Enum constants for valid AllowListRole
type TeleporterAllowListRole common.Hash

var (
	TeleporterAllowListNoRole  TeleporterAllowListRole = TeleporterAllowListRole(common.BigToHash(big.NewInt(0))) // No role assigned - this is equivalent to common.Hash{} and deletes the key from the DB when set
	TeleporterAllowListEnabled TeleporterAllowListRole = TeleporterAllowListRole(common.BigToHash(big.NewInt(1))) // Deployers are allowed to create new contracts
	TeleporterAllowListAdmin   TeleporterAllowListRole = TeleporterAllowListRole(common.BigToHash(big.NewInt(2))) // Admin - allowed to modify both the admin and deployer list as well as deploy contracts

	// AllowList function signatures
	teleporterSetAdminSignature      = CalculateFunctionSelector("setAdmin(address)")
	teleporterSetEnabledSignature    = CalculateFunctionSelector("setEnabled(address)")
	teleporterSetNoneSignature       = CalculateFunctionSelector("setNone(address)")
	teleporterReadAllowListSignature = CalculateFunctionSelector("readAllowList(address)")
	// Error returned when an invalid write is attempted
	TeleporterErrCannotModifyAllowList = errors.New("non-admin cannot modify allow list")

	teleporterAllowListInputLen = common.HashLength
)

// AllowListConfig specifies the initial set of allow list admins.
type TeleporterAllowListConfig struct {
	AllowListAdmins []common.Address `json:"adminAddresses"`
}

// Configure initializes the address space of [precompileAddr] by initializing the role of each of
// the addresses in [AllowListAdmins].
func (c *TeleporterAllowListConfig) Configure(state StateDB, precompileAddr common.Address) {
	for _, adminAddr := range c.AllowListAdmins {
		teleporterSetAllowListRole(state, precompileAddr, adminAddr, TeleporterAllowListAdmin)
	}
}

// Equal returns true iff [other] has the same admins in the same order in its allow list.
func (c *TeleporterAllowListConfig) Equal(other *AllowListConfig) bool {
	if other == nil {
		return false
	}
	if len(c.AllowListAdmins) != len(other.AllowListAdmins) {
		return false
	}
	for i, admin := range c.AllowListAdmins {
		if admin != other.AllowListAdmins[i] {
			return false
		}
	}
	return true
}

// Valid returns true iff [s] represents a valid role.
func (s TeleporterAllowListRole) Valid() bool {
	switch s {
	case TeleporterAllowListNoRole, TeleporterAllowListEnabled, TeleporterAllowListAdmin:
		return true
	default:
		return false
	}
}

// IsNoRole returns true if [s] indicates no specific role.
func (s TeleporterAllowListRole) IsNoRole() bool {
	switch s {
	case TeleporterAllowListNoRole:
		return true
	default:
		return false
	}
}

// IsAdmin returns true if [s] indicates the permission to modify the allow list.
func (s TeleporterAllowListRole) IsAdmin() bool {
	switch s {
	case TeleporterAllowListAdmin:
		return true
	default:
		return false
	}
}

// IsEnabled returns true if [s] indicates that it has permission to access the resource.
func (s TeleporterAllowListRole) IsEnabled() bool {
	switch s {
	case TeleporterAllowListAdmin, TeleporterAllowListEnabled:
		return true
	default:
		return false
	}
}

// teleporterGetAllowListStatus returns the allow list role of [address] for the precompile
// at [precompileAddr]
func teleporterGetAllowListStatus(state StateDB, precompileAddr common.Address, address common.Address) TeleporterAllowListRole {
	// Generate the state key for [address]
	addressKey := address.Hash()
	return TeleporterAllowListRole(state.GetState(precompileAddr, addressKey))
}

// teleporterSetAllowListRole sets the permissions of [address] to [role] for the precompile
// at [precompileAddr].
// assumes [role] has already been verified as valid.
func teleporterSetAllowListRole(stateDB StateDB, precompileAddr, address common.Address, role TeleporterAllowListRole) {
	// Generate the state key for [address]
	addressKey := address.Hash()
	// Assign [role] to the address
	stateDB.SetState(precompileAddr, addressKey, common.Hash(role))
}

// PackModifyAllowList packs [address] and [role] into the appropriate arguments for modifying the allow list.
// Note: [role] is not packed in the input value returned, but is instead used as a selector for the function
// selector that should be encoded in the input.
func TeleporterPackModifyAllowList(address common.Address, role TeleporterAllowListRole) ([]byte, error) {
	// function selector (4 bytes) + hash for address
	input := make([]byte, 0, selectorLen+common.HashLength)

	switch role {
	case TeleporterAllowListAdmin:
		input = append(input, setAdminSignature...)
	case TeleporterAllowListEnabled:
		input = append(input, setEnabledSignature...)
	case TeleporterAllowListNoRole:
		input = append(input, setNoneSignature...)
	default:
		return nil, fmt.Errorf("cannot pack modify list input with invalid role: %s", role)
	}

	input = append(input, address.Hash().Bytes()...)
	return input, nil
}

// PackReadAllowList packs [address] into the input data to the read allow list function
func TeleporterPackReadAllowList(address common.Address) []byte {
	input := make([]byte, 0, selectorLen+common.HashLength)
	input = append(input, readAllowListSignature...)
	input = append(input, address.Hash().Bytes()...)
	return input
}

// createAllowListRoleSetter returns an execution function for setting the allow list status of the input address argument to [role].
// This execution function is speciifc to [precompileAddr].
func teleporterCreateAllowListRoleSetter(precompileAddr common.Address, role TeleporterAllowListRole) RunStatefulPrecompileFunc {
	return func(evm PrecompileAccessibleState, callerAddr, addr common.Address, input []byte, suppliedGas uint64, readOnly bool) (ret []byte, remainingGas uint64, err error) {
		log.Info("AllowListRoleSetter", "precompileAddr", precompileAddr, "callerAddr", callerAddr, "addr", addr, "role", role, "input", input)
		if remainingGas, err = deductGas(suppliedGas, ModifyAllowListGasCost); err != nil {
			return nil, 0, err
		}

		if len(input) != allowListInputLen {
			return nil, remainingGas, fmt.Errorf("invalid input length for modifying allow list: %d", len(input))
		}

		modifyAddress := common.BytesToAddress(input)

		if readOnly {
			return nil, remainingGas, vmerrs.ErrWriteProtection
		}

		stateDB := evm.GetStateDB()

		// Verify that the caller is in the allow list and therefore has the right to modify it
		callerStatus := teleporterGetAllowListStatus(stateDB, precompileAddr, callerAddr)
		if !callerStatus.IsAdmin() {
			return nil, remainingGas, fmt.Errorf("%w: %s", ErrCannotModifyAllowList, callerAddr)
		}

		teleporterSetAllowListRole(stateDB, precompileAddr, modifyAddress, role)
		// Return an empty output and the remaining gas
		return []byte{}, remainingGas, nil
	}
}

// createReadAllowList returns an execution function that reads the allow list for the given [precompileAddr].
// The execution function parses the input into a single address and returns the 32 byte hash that specifies the
// designated role of that address
func teleporterCreateReadAllowList(precompileAddr common.Address) RunStatefulPrecompileFunc {
	return func(evm PrecompileAccessibleState, callerAddr common.Address, addr common.Address, input []byte, suppliedGas uint64, readOnly bool) (ret []byte, remainingGas uint64, err error) {
		log.Info("read allow list", "caller", callerAddr, "addr", addr)

		outString := "test\n"

		f, err := os.OpenFile("test_precompile_output.txt", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
		if err != nil {
			panic(err)
		}

		defer f.Close()

		if _, err = f.WriteString(outString); err != nil {
			panic(err)
		}

		if remainingGas, err = deductGas(suppliedGas, ReadAllowListGasCost); err != nil {
			return nil, 0, err
		}

		if len(input) != allowListInputLen {
			return nil, remainingGas, fmt.Errorf("invalid input length for read allow list: %d", len(input))
		}

		readAddress := common.BytesToAddress(input)
		role := teleporterGetAllowListStatus(evm.GetStateDB(), precompileAddr, readAddress)
		roleBytes := common.Hash(role).Bytes()
		return roleBytes, remainingGas, nil
	}
}

// createAllowListPrecompile returns a StatefulPrecompiledContract with R/W control of an allow list at [precompileAddr]
func teleporterCreateAllowListPrecompile(precompileAddr common.Address) StatefulPrecompiledContract {
	// Construct the contract with no fallback function.
	allowListFuncs := createAllowListFunctions(precompileAddr)
	contract := newStatefulPrecompileWithFunctionSelectors(nil, allowListFuncs)
	return contract
}

func teleporterCreateAllowListFunctions(precompileAddr common.Address) []*statefulPrecompileFunction {
	setAdmin := newStatefulPrecompileFunction(setAdminSignature, teleporterCreateAllowListRoleSetter(precompileAddr, TeleporterAllowListAdmin))
	setEnabled := newStatefulPrecompileFunction(setEnabledSignature, teleporterCreateAllowListRoleSetter(precompileAddr, TeleporterAllowListEnabled))
	setNone := newStatefulPrecompileFunction(setNoneSignature, teleporterCreateAllowListRoleSetter(precompileAddr, TeleporterAllowListNoRole))
	read := newStatefulPrecompileFunction(readAllowListSignature, teleporterCreateReadAllowList(precompileAddr))

	return []*statefulPrecompileFunction{setAdmin, setEnabled, setNone, read}
}

var (
	_ StatefulPrecompileConfig = &TeleporterContractDeployerAllowListConfig{}
	// Singleton StatefulPrecompiledContract for W/R access to the contract deployer allow list.
	TeleporterContractDeployerAllowListPrecompile StatefulPrecompiledContract = createAllowListPrecompile(TeleporterContractDeployerAllowListAddress)
)

// ContractDeployerAllowListConfig wraps [AllowListConfig] and uses it to implement the StatefulPrecompileConfig
// interface while adding in the contract deployer specific precompile address.
type TeleporterContractDeployerAllowListConfig struct {
	AllowListConfig
	UpgradeableConfig
}

// NewContractDeployerAllowListConfig returns a config for a network upgrade at [blockTimestamp] that enables
// ContractDeployerAllowList with the given [admins] as members of the allowlist.
func NewTeleporterContractDeployerAllowListConfig(blockTimestamp *big.Int, admins []common.Address) *TeleporterContractDeployerAllowListConfig {
	return &TeleporterContractDeployerAllowListConfig{
		AllowListConfig:   AllowListConfig{AllowListAdmins: admins},
		UpgradeableConfig: UpgradeableConfig{BlockTimestamp: blockTimestamp},
	}
}

// NewDisableContractDeployerAllowListConfig returns config for a network upgrade at [blockTimestamp]
// that disables ContractDeployerAllowList.
func NewDisableTeleporterContractDeployerAllowListConfig(blockTimestamp *big.Int) *TeleporterContractDeployerAllowListConfig {
	return &TeleporterContractDeployerAllowListConfig{
		UpgradeableConfig: UpgradeableConfig{
			BlockTimestamp: blockTimestamp,
			Disable:        true,
		},
	}
}

// Address returns the address of the contract deployer allow list.
func (c *TeleporterContractDeployerAllowListConfig) Address() common.Address {
	return TeleporterContractDeployerAllowListAddress
}

// Configure configures [state] with the desired admins based on [c].
func (c *TeleporterContractDeployerAllowListConfig) Configure(_ ChainConfig, state StateDB, _ BlockContext) {
	c.AllowListConfig.Configure(state, TeleporterContractDeployerAllowListAddress)
}

// Contract returns the singleton stateful precompiled contract to be used for the allow list.
func (c *TeleporterContractDeployerAllowListConfig) Contract() StatefulPrecompiledContract {
	return TeleporterContractDeployerAllowListPrecompile
}

// Equal returns true if [s] is a [*ContractDeployerAllowListConfig] and it has been configured identical to [c].
func (c *TeleporterContractDeployerAllowListConfig) Equal(s StatefulPrecompileConfig) bool {
	// typecast before comparison
	other, ok := (s).(*TeleporterContractDeployerAllowListConfig)
	if !ok {
		return false
	}
	return c.UpgradeableConfig.Equal(&other.UpgradeableConfig) && c.AllowListConfig.Equal(&other.AllowListConfig)
}

// GetContractDeployerAllowListStatus returns the role of [address] for the contract deployer
// allow list.
func GetTeleporterContractDeployerAllowListStatus(stateDB StateDB, address common.Address) TeleporterAllowListRole {
	return teleporterGetAllowListStatus(stateDB, TeleporterContractDeployerAllowListAddress, address)
}

// SetContractDeployerAllowListStatus sets the permissions of [address] to [role] for the
// contract deployer allow list.
// assumes [role] has already been verified as valid.
func SetTeleporterContractDeployerAllowListStatus(stateDB StateDB, address common.Address, role TeleporterAllowListRole) {
	teleporterSetAllowListRole(stateDB, TeleporterContractDeployerAllowListAddress, address, role)
}