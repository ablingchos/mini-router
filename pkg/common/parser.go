package common

import (
	"bytes"
	"io"
	"os"

	"git.woa.com/mfcn/ms-go/pkg/util"
	yamltool "github.com/ghodss/yaml"
	"gopkg.in/yaml.v2"
)

func LoadYAML(path string) ([][]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, util.ErrorWithPos(err)
	}
	parts, err := splitYAML(data)
	if err != nil {
		return nil, util.ErrorWithPos(err)
	}
	resp := [][]byte{}
	for _, part := range parts {
		temp, err := yamltool.YAMLToJSON(part)
		if err != nil {
			return nil, util.ErrorWithPos(err)
		}
		resp = append(resp, temp)
	}
	return resp, nil
}

// SplitYAML is used to split yaml
func splitYAML(data []byte) ([][]byte, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var parts [][]byte
	for {
		var value interface{}
		err := decoder.Decode(&value)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		part, err := yaml.Marshal(value)
		if err != nil {
			return nil, err
		}
		parts = append(parts, part)
	}
	return parts, nil
}
