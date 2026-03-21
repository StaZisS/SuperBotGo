package hostapi

import "strings"

type PermissionInfo struct {
	Key         string `json:"key"`
	Description string `json:"description"`
	Category    string `json:"category"`
}

func AllHostPermissions() []PermissionInfo {
	return []PermissionInfo{
		{Key: "db:read", Description: "Чтение данных из БД (db_query)", Category: "database"},
		{Key: "db:write", Description: "Запись данных в БД (db_save)", Category: "database"},
		{Key: "network:read", Description: "HTTP-запросы GET/HEAD", Category: "network"},
		{Key: "network:write", Description: "HTTP-запросы POST/PUT/DELETE и др.", Category: "network"},
		{Key: "plugins:events", Description: "Публикация событий (publish_event)", Category: "plugins"},
		{Key: "triggers:http", Description: "Прием HTTP-запросов (webhooks, API)", Category: "triggers"},
		{Key: "triggers:cron", Description: "Запуск по расписанию (cron)", Category: "triggers"},
		{Key: "triggers:events", Description: "Подписка на события других плагинов", Category: "triggers"},
	}
}

func IsKnownPermission(key string) bool {
	for _, p := range AllHostPermissions() {
		if p.Key == key {
			return true
		}
	}
	return strings.HasPrefix(key, "plugins:call:") && len(key) > len("plugins:call:")
}
