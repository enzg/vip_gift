package pkg

import (
	"encoding/json"
	"log"
)

// mergeJSON - 将 oldJSON 解析后, 与 extraMap 合并, 再序列化回字符串
func MergeJSON(oldJSON string, extraMap map[string]interface{}) string {
	merged := make(map[string]interface{})
	// 先解析 oldJSON
	if oldJSON != "" {
		if err := json.Unmarshal([]byte(oldJSON), &merged); err != nil {
			log.Printf("[mergeJSON] unmarshal oldJSON error: %v\n", err)
		}
	}
	// 合并 extraMap
	for k, v := range extraMap {
		merged[k] = v
	}
	// 再序列化
	newBytes, err := json.Marshal(merged)
	if err != nil {
		log.Printf("[mergeJSON] marshal merged error: %v\n", err)
		return oldJSON // 出错就返回原字符串
	}
	return string(newBytes)
}
