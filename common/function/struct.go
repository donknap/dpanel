package function

import "encoding/json"

func StructToMap(obj interface{}) map[string]interface{} {
	b, _ := json.Marshal(obj)
	var m map[string]interface{}
	_ = json.Unmarshal(b, &m)
	return m
}
