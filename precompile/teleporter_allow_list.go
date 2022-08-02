// (c) 2019-2020, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package precompile

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

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
func GetTeleporterContractDeployerAllowListStatus(stateDB StateDB, address common.Address) AllowListRole {
	return getAllowListStatus(stateDB, TeleporterContractDeployerAllowListAddress, address)
}

// SetContractDeployerAllowListStatus sets the permissions of [address] to [role] for the
// contract deployer allow list.
// assumes [role] has already been verified as valid.
func SetTeleporterContractDeployerAllowListStatus(stateDB StateDB, address common.Address, role AllowListRole) {
	setAllowListRole(stateDB, TeleporterContractDeployerAllowListAddress, address, role)
}
