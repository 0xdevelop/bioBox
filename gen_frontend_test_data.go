//go:build ignore

// gen_frontend_test_data.go 为前端构建生成默认输入，本地 PXDesign 输入可选。
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const (
	protenixSource = "example_files/test_data_protenix.json"
	pxdesignSource = "example_files/test_data_pxdesign.json"
	generatedPath  = "bb_frontend/src/generated_test_data.ts"
)

func main() {
	protenix, err := os.ReadFile(protenixSource)
	if err != nil {
		fail(err.Error())
	}
	if err = validateProtenixInput(protenix); err != nil {
		fail(protenixSource + ": " + err.Error())
	}
	pxdesignName := ""
	pxdesign, err := os.ReadFile(pxdesignSource)
	if err == nil {
		if err = validatePXDesignInput(pxdesign); err != nil {
			fail(pxdesignSource + ": " + err.Error())
		}
		pxdesignName = "test_data_pxdesign.json"
	} else if !os.IsNotExist(err) {
		fail(err.Error())
	}
	body := renderFile("protenixDefaultFile", "test_data_protenix.json", protenix) +
		renderFile("pxdesignDefaultFile", pxdesignName, pxdesign)
	if err = os.WriteFile(generatedPath, []byte(body), 0o644); err != nil {
		fail(err.Error())
	}
	fmt.Printf("[gen_frontend_test_data] wrote %s\n", generatedPath)
}

func fail(message string) {
	fmt.Fprintf(os.Stderr, "[gen_frontend_test_data] FAIL: %s\n", message)
	os.Exit(1)
}

func validateProtenixInput(raw []byte) error {
	var tasks []map[string]any
	if err := json.Unmarshal(raw, &tasks); err != nil {
		return err
	}
	if len(tasks) != 1 {
		return fmt.Errorf("expected one task, got %d", len(tasks))
	}
	name, _ := tasks[0]["name"].(string)
	if name != "user_test" {
		return fmt.Errorf("task name %q is not the official user_test example", name)
	}
	sequences, _ := tasks[0]["sequences"].([]any)
	if len(sequences) != 1 {
		return fmt.Errorf("expected one sequence entry")
	}
	entry, _ := sequences[0].(map[string]any)
	chain, _ := entry["proteinChain"].(map[string]any)
	sequence, _ := chain["sequence"].(string)
	if strings.TrimSpace(sequence) == "" {
		return fmt.Errorf("proteinChain.sequence is empty")
	}
	if _, ok := chain["count"]; !ok {
		return fmt.Errorf("proteinChain.count is missing")
	}
	return nil
}

func validatePXDesignInput(raw []byte) error {
	var tasks []map[string]any
	if err := json.Unmarshal(raw, &tasks); err != nil {
		return err
	}
	if len(tasks) != 1 || len(tasks[0]) == 0 {
		return fmt.Errorf("expected one nonempty PXDesign task")
	}
	return nil
}

func renderFile(exportName, fileName string, raw []byte) string {
	var buf bytes.Buffer
	if exportName == "protenixDefaultFile" {
		buf.WriteString("// 由 gen_frontend_test_data.go 从 example_files 的真实输入文件生成。不要手改。\n")
	}
	fmt.Fprintf(&buf, "export const %s = {\n", exportName)
	fmt.Fprintf(&buf, "  name: %s,\n", jsString(fileName))
	fmt.Fprintf(&buf, "  text: %s,\n", jsString(string(raw)))
	buf.WriteString("};\n\n")
	return buf.String()
}

func jsString(value string) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		fail(err.Error())
	}
	return string(encoded)
}
