// (c) 2019-2020, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package precompile

import (
	"errors"
	"fmt"
	"math/big"
	"os"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

// Enum constants for valid AllowListRole
type TeleporterAllowListRole common.Hash

var (
	TeleporterAllowListAdmin TeleporterAllowListRole = TeleporterAllowListRole(common.BigToHash(big.NewInt(2))) // Admin - allowed to modify both the admin and deployer list as well as deploy contracts

	// AllowList function signatures
	testFunctionSignature = CalculateFunctionSelector("testFunction(address)")
	// Error returned when an invalid write is attempted
	TeleporterErrCannotModifyAllowList = errors.New("non-admin cannot modify allow list")

	_ StatefulPrecompileConfig = &TeleporterConfig{}
	// Singleton StatefulPrecompiledContract for W/R access to the contract deployer allow list.
	TeleporterPrecompile StatefulPrecompiledContract = createTeleporterPrecompile(TeleporterAddress)
)

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

// createTestFunction returns an execution function that reads the allow list for the given [precompileAddr].
// The execution function parses the input into a single address and returns the 32 byte hash that specifies the
// designated role of that address
func createTestFunction(precompileAddr common.Address) RunStatefulPrecompileFunc {
	return func(evm PrecompileAccessibleState, callerAddr common.Address, addr common.Address, input []byte, suppliedGas uint64, readOnly bool) (ret []byte, remainingGas uint64, err error) {
		log.Info("testFunction", "caller", callerAddr, "addr", addr)

		outString := "test 1\n"

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

// createTeleporterPrecompile returns a StatefulPrecompiledContract with R/W control of an allow list at [precompileAddr]
func createTeleporterPrecompile(precompileAddr common.Address) StatefulPrecompiledContract {
	// Construct the contract with no fallback function.
	allowListFuncs := createTeleporterFunctions(precompileAddr)
	contract := newStatefulPrecompileWithFunctionSelectors(nil, allowListFuncs)
	return contract
}

func createTeleporterFunctions(precompileAddr common.Address) []*statefulPrecompileFunction {
	read := newStatefulPrecompileFunction(testFunctionSignature, createTestFunction(precompileAddr))

	return []*statefulPrecompileFunction{read}
}

// TeleporterConfig wraps [TeleporterConfig] and uses it to implement the StatefulPrecompileConfig
// interface while adding in the contract deployer specific precompile address.
type TeleporterConfig struct {
	AllowListAdmins []common.Address `json:"adminAddresses"`
	UpgradeableConfig
}

// NewTeleporterConfig returns a config for a network upgrade at [blockTimestamp] that enables
// ContractDeployerAllowList with the given [admins] as members of the allowlist.
func NewTeleporterConfig(blockTimestamp *big.Int, admins []common.Address) *TeleporterConfig {
	return &TeleporterConfig{
		AllowListAdmins:   admins,
		UpgradeableConfig: UpgradeableConfig{BlockTimestamp: blockTimestamp},
	}
}

// NewDisableTeleporterConfig returns config for a network upgrade at [blockTimestamp]
// that disables Teleporter.
func NewDisableTeleporterConfig(blockTimestamp *big.Int) *TeleporterConfig {
	return &TeleporterConfig{
		UpgradeableConfig: UpgradeableConfig{
			BlockTimestamp: blockTimestamp,
			Disable:        true,
		},
	}
}

// Address returns the address of the contract deployer allow list.
func (c *TeleporterConfig) Address() common.Address {
	return TeleporterAddress
}

// Configure configures [state] with the desired admins based on [c].
func (c *TeleporterConfig) Configure(_ ChainConfig, state StateDB, _ BlockContext) {
	for _, adminAddr := range c.AllowListAdmins {
		teleporterSetAllowListRole(state, TeleporterAddress, adminAddr, TeleporterAllowListAdmin)
	}
}

// Contract returns the singleton stateful precompiled contract to be used for the allow list.
func (c *TeleporterConfig) Contract() StatefulPrecompiledContract {
	return TeleporterPrecompile
}

// Equal returns true if [s] is a [*ContractDeployerAllowListConfig] and it has been configured identical to [c].
func (c *TeleporterConfig) Equal(s StatefulPrecompileConfig) bool {
	// typecast before comparison
	other, ok := (s).(*TeleporterConfig)
	if !ok {
		return false
	}
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
	return c.UpgradeableConfig.Equal(&other.UpgradeableConfig)
}
