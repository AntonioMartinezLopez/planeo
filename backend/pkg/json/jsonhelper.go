package jsonhelper

import "encoding/json"

func ServeJson(data interface{}) []byte {
	value, _ := json.Marshal(data)
	if value != nil {
		return value
	}
	return nil
}
