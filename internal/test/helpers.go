package test

import "fmt"

func AsMap(kv ...string) map[string]string {
	if len(kv)%2 != 0 {
		panic(fmt.Errorf("cannot convert to Map"))
	}

	m := make(map[string]string, len(kv)/2)
	for i := 0; i < len(kv); i += 2 {
		k := kv[i]
		v := kv[i+1]
		m[k] = v
	}

	return m
}
