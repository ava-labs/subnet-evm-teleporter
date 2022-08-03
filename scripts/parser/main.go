// (c) 2022 Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"io/ioutil"
	"os"
	"strings"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/subnet-evm/tests/e2e/runner"
	"github.com/ava-labs/subnet-evm/tests/e2e/utils"
	"github.com/ethereum/go-ethereum/log"
	"github.com/fatih/color"
	"gopkg.in/yaml.v2"
)

/*
===Example File===

endpoint: /ext/bc/2Z36RnQuk1hvsnFeGWzfZUfXNr7w1SjzmDQ78YxfTVNAkDq3nZ
logsDir: /var/folders/mp/6jm81gc11dv3xtcwxmrd8mcr0000gn/T/runnerlogs2984620995
pid: 55547
uris:
- http://localhost:61278
- http://localhost:61280
- http://localhost:61282
- http://localhost:61284
- http://localhost:61286
*/

type output struct {
	Endpoint string   `yaml:"endpoint"`
	Logs     string   `yaml:"logsDir"`
	URIs     []string `yaml:"uris"`
}

func startSubnet(outputFile string, avalanchegoPath string, pluginDir string, genesisPath string) {
	// log the genesisPath to stdout
	log.Info("startSubnet", "genesisPath", genesisPath)
	// set vmid
	bytes := make([]byte, 32)
	vmName := "subnetevm"
	copy(bytes, []byte(vmName))
	var err error
	vmId, err := ids.ToID(bytes)
	if err != nil {
		panic(err)
	}

	// Start subnet-evm A
	// This cannot resolve relative paths for the genesis file
	_, err = runner.StartNetwork(vmId, vmName, genesisPath, pluginDir)
	if err != nil {
		panic(err)
	}

	// Wait for A
	blockchainId, logsDir, err := runner.WaitForCustomVm(vmId)
	if err != nil {
		panic(err)
	}
	runner.GetClusterInfo(blockchainId, logsDir)
}

func parseMetamask(outputFile string, chainId string, address string) {
	yamlFile, err := ioutil.ReadFile(outputFile)
	if err != nil {
		panic(err)
	}
	var o output
	if err := yaml.Unmarshal(yamlFile, &o); err != nil {
		panic(err)
	}

	color.Green("\n")
	color.Green("Logs Directory: %s", o.Logs)
	color.Green("\n")

	color.Green("EVM Chain ID: %s", chainId)
	color.Green("Funded Address: %s", address)
	color.Green("RPC Endpoints:")
	for _, uri := range o.URIs {
		color.Green("- %s%s/rpc", uri, o.Endpoint)
	}
	color.Green("\n")

	color.Green("WS Endpoints:")
	for _, uri := range o.URIs {
		wsURI := strings.ReplaceAll(uri, "http", "ws")
		color.Green("- %s%s/ws", wsURI, o.Endpoint)
	}
	color.Green("\n")

	color.Yellow("MetaMask Quick Start:")
	color.Yellow("Funded Address: %s", address)
	color.Yellow("Network Name: Local EVM")
	color.Yellow("RPC URL: %s%s/rpc", o.URIs[0], o.Endpoint)
	color.Yellow("Chain ID: %s", chainId)
	color.Yellow("Currency Symbol: LEVM")
}

func main() {
	if len(os.Args) != 8 {
		panic("missing args <yaml> <chainID> <address> <avalanchego-path> <plugin-dir> <grpc-endpoint> <genesis-path>")
	}

	outputFile := os.Args[1]
	//chainId := os.Args[2]
	address := os.Args[3]
	avagoPath := os.Args[4]
	pluginDir := os.Args[5]
	grpc := os.Args[6]
	log.Info("main", "grpcval", grpc)
	//genesis := os.Args[7]

	var err error
	utils.SetOutputFile(outputFile)
	utils.SetPluginDir(pluginDir)
	err = runner.InitializeRunner(avagoPath, grpc, "info")
	if err != nil {
		panic(err)
	}

	genesisPathA := "./genesisA.json"
	chainIdA := "99999"
	startSubnet(outputFile, avagoPath, pluginDir, genesisPathA)
	parseMetamask(outputFile, chainIdA, address)

	genesisPathB := "./genesisB.json"
	chainIdB := "99991"
	startSubnet(outputFile, avagoPath, pluginDir, genesisPathB)
	parseMetamask(outputFile, chainIdB, address)
}
