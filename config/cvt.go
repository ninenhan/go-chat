package config

import (
	"encoding/json"
	"errors"
	"fmt"
)

func AutoJSONDecode[T any](raw string) (*T, error) {
	var target T
	data := []byte(raw)

	// 最多解 5 层（已经足够，大部分 1~2 层）
	for i := 0; i < 5; i++ {
		// 尝试直接解析为目标结构
		if err := json.Unmarshal(data, &target); err == nil {
			return &target, nil
		}

		// 尝试解析为 *string（表示存在转义层）
		var inner string
		if err := json.Unmarshal(data, &inner); err != nil {
			return nil, fmt.Errorf("decode failed level=%d: %w", i, err)
		}
		// 进入下一层
		data = []byte(inner)
	}
	return nil, errors.New("too many escape layers (limit=5)")
}
