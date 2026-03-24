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
		{Key: "kv:read", Description: "Чтение из key-value хранилища (kv_get, kv_list)", Category: "kv"},
		{Key: "kv:write", Description: "Запись в key-value хранилище (kv_set, kv_delete)", Category: "kv"},
		{Key: "notify:user", Description: "Отправка уведомлений пользователю (notify_user)", Category: "notifications"},
		{Key: "notify:chat", Description: "Отправка уведомлений в чат (notify_chat)", Category: "notifications"},
		{Key: "notify:project", Description: "Рассылка уведомлений по проекту (notify_project)", Category: "notifications"},
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
