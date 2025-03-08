package vconfig

import (
	"fmt"
	"reflect"
	"strings"
)

// findConfigChanges 查找两个值之间的差异，返回变更的配置项列表
func findConfigChanges(oldData, newData interface{}, path string) []ConfigChangedItem {
	var changes []ConfigChangedItem

	oldVal := reflect.ValueOf(oldData)
	newVal := reflect.ValueOf(newData)

	// 处理指针类型
	if oldVal.Kind() == reflect.Ptr {
		oldVal = oldVal.Elem()
	}
	if newVal.Kind() == reflect.Ptr {
		newVal = newVal.Elem()
	}

	// 处理nil值或无效值
	if !oldVal.IsValid() && !newVal.IsValid() {
		return changes // 两者都无效，无变化
	}
	if !oldVal.IsValid() {
		// 旧值无效，新值有效，视为新增
		return []ConfigChangedItem{{
			Path:     path,
			OldValue: nil,
			NewValue: newData,
		}}
	}
	if !newVal.IsValid() {
		// 旧值有效，新值无效，视为删除
		return []ConfigChangedItem{{
			Path:     path,
			OldValue: oldData,
			NewValue: nil,
		}}
	}

	// 如果类型不同，直接认为整个值都变了
	if oldVal.Type() != newVal.Type() {
		return []ConfigChangedItem{{
			Path:     path,
			OldValue: oldData,
			NewValue: newData,
		}}
	}

	switch oldVal.Kind() {
	case reflect.Struct:
		// 先比较整体是否相等
		if reflect.DeepEqual(oldVal.Interface(), newVal.Interface()) {
			return changes // 完全相等，无变化
		}

		// 遍历结构体的每个字段
		for i := 0; i < oldVal.NumField(); i++ {
			fieldName := oldVal.Type().Field(i).Name
			oldField := oldVal.Field(i)
			newField := newVal.Field(i)

			// 如果字段不可比较，跳过
			if !oldField.CanInterface() || !newField.CanInterface() {
				continue
			}

			// 获取字段的tag名称（如果有）
			tag := oldVal.Type().Field(i).Tag
			yamlTag := tag.Get("yaml")
			jsonTag := tag.Get("json")
			fieldPath := fieldName
			if yamlTag != "" && yamlTag != "-" {
				parts := strings.Split(yamlTag, ",")
				fieldPath = parts[0]
			} else if jsonTag != "" && jsonTag != "-" {
				parts := strings.Split(jsonTag, ",")
				fieldPath = parts[0]
			}

			// 组合完整路径
			fullPath := path
			if fullPath != "" {
				fullPath += "."
			}
			fullPath += fieldPath

			// 递归比较字段值
			if oldField.Kind() == reflect.Struct || oldField.Kind() == reflect.Map ||
				oldField.Kind() == reflect.Slice || oldField.Kind() == reflect.Array {
				// 复杂类型递归比较
				fieldChanges := findConfigChanges(oldField.Interface(), newField.Interface(), fullPath)
				if len(fieldChanges) > 0 {
					changes = append(changes, fieldChanges...)
				}
			} else if !reflect.DeepEqual(oldField.Interface(), newField.Interface()) {
				// 基本类型直接比较
				changes = append(changes, ConfigChangedItem{
					Path:     fullPath,
					OldValue: oldField.Interface(),
					NewValue: newField.Interface(),
				})
			}
		}

	case reflect.Map:
		// 先比较整体是否相等
		if reflect.DeepEqual(oldVal.Interface(), newVal.Interface()) {
			return changes // 完全相等，无变化
		}

		// 获取所有的键
		allKeys := make(map[interface{}]bool)
		for _, key := range oldVal.MapKeys() {
			allKeys[key.Interface()] = true
		}
		for _, key := range newVal.MapKeys() {
			allKeys[key.Interface()] = true
		}

		// 比较每个键对应的值
		for key := range allKeys {
			keyVal := reflect.ValueOf(key)
			oldMapVal := oldVal.MapIndex(keyVal)
			newMapVal := newVal.MapIndex(keyVal)

			keyStr := fmt.Sprintf("%v", key)
			fullPath := path
			if fullPath != "" {
				fullPath += "."
			}
			fullPath += keyStr

			if !oldMapVal.IsValid() {
				// 新增的键
				changes = append(changes, ConfigChangedItem{
					Path:     fullPath,
					OldValue: nil,
					NewValue: newMapVal.Interface(),
				})
			} else if !newMapVal.IsValid() {
				// 删除的键
				changes = append(changes, ConfigChangedItem{
					Path:     fullPath,
					OldValue: oldMapVal.Interface(),
					NewValue: nil,
				})
			} else if oldMapVal.Kind() == reflect.Map || oldMapVal.Kind() == reflect.Struct ||
				oldMapVal.Kind() == reflect.Slice || oldMapVal.Kind() == reflect.Array {
				// 复杂类型递归比较
				fieldChanges := findConfigChanges(oldMapVal.Interface(), newMapVal.Interface(), fullPath)
				if len(fieldChanges) > 0 {
					changes = append(changes, fieldChanges...)
				}
			} else if !reflect.DeepEqual(oldMapVal.Interface(), newMapVal.Interface()) {
				// 基本类型直接比较值
				changes = append(changes, ConfigChangedItem{
					Path:     fullPath,
					OldValue: oldMapVal.Interface(),
					NewValue: newMapVal.Interface(),
				})
			}
		}

	case reflect.Slice, reflect.Array:
		// 先比较整体是否相等
		if reflect.DeepEqual(oldVal.Interface(), newVal.Interface()) {
			return changes // 完全相等，无变化
		}

		// 如果长度不同，直接认为整个数组/切片都变了
		if oldVal.Len() != newVal.Len() {
			changes = append(changes, ConfigChangedItem{
				Path:     path,
				OldValue: oldVal.Interface(),
				NewValue: newVal.Interface(),
			})
			return changes
		}

		// 比较每个元素
		for i := 0; i < oldVal.Len(); i++ {
			oldItem := oldVal.Index(i)
			newItem := newVal.Index(i)

			// 如果元素不可比较，跳过
			if !oldItem.CanInterface() || !newItem.CanInterface() {
				continue
			}

			itemPath := fmt.Sprintf("%s[%d]", path, i)

			if oldItem.Kind() == reflect.Map || oldItem.Kind() == reflect.Struct ||
				oldItem.Kind() == reflect.Slice || oldItem.Kind() == reflect.Array {
				// 复杂类型递归比较
				itemChanges := findConfigChanges(oldItem.Interface(), newItem.Interface(), itemPath)
				if len(itemChanges) > 0 {
					changes = append(changes, itemChanges...)
				}
			} else if !reflect.DeepEqual(oldItem.Interface(), newItem.Interface()) {
				// 基本类型直接比较值
				changes = append(changes, ConfigChangedItem{
					Path:     itemPath,
					OldValue: oldItem.Interface(),
					NewValue: newItem.Interface(),
				})
			}
		}

		// 如果没有发现元素级别的变化，但整体不相等（可能是元素顺序变了），记录整体变化
		if len(changes) == 0 && !reflect.DeepEqual(oldVal.Interface(), newVal.Interface()) {
			changes = append(changes, ConfigChangedItem{
				Path:     path,
				OldValue: oldVal.Interface(),
				NewValue: newVal.Interface(),
			})
		}

	default:
		// 基本类型，直接比较值
		if !reflect.DeepEqual(oldVal.Interface(), newVal.Interface()) {
			changes = append(changes, ConfigChangedItem{
				Path:     path,
				OldValue: oldVal.Interface(),
				NewValue: newVal.Interface(),
			})
		}
	}

	return changes
}
