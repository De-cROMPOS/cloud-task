package balancer

import (
	"encoding/json"
	"fmt"
	"os"
)

var (
	configPath = "config/cfg.json"
)

type ConfigData struct {
	Port    string   `json:"port"`
	Servers []string `json:"server"`
}

func NewConfigData() *ConfigData {
	return &ConfigData{
		Port:    "",
		Servers: make([]string, 0),
	}
}

//getting data from config file to transfer it to tlhe balancer
func (cd *ConfigData) GetCfgData() error {
	file, err := os.Open(configPath)
	if err != nil {
		return fmt.Errorf("config file can't be open: %v", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(cd); err != nil {
		return fmt.Errorf("error while decoding cfg: %v", err)
	}

	return nil
}
