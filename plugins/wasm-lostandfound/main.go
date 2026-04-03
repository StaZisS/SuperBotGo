package main

import (
	"embed"
	"fmt"
	"strconv"
	"strings"

	"github.com/superbot/wasmplugin"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

//go:embed i18n/*.toml
var i18nFS embed.FS

var cat = wasmplugin.NewCatalog("en").
	LoadFS(i18nFS, "i18n")

func main() {
	wasmplugin.Run(wasmplugin.Plugin{
		ID:      "lostandfound",
		Name:    "Lost & Found",
		Version: "1.0.0",
		Requirements: []wasmplugin.Requirement{
			wasmplugin.Database("Store lost item records").Build(),
			wasmplugin.File("Store and serve item photos").Build(),
		},
		Migrations: wasmplugin.MigrationsFromFS(migrationsFS, "migrations"),
		Triggers: []wasmplugin.Trigger{
			addCommand(),
			listCommand(),
			detailCommand(),
			myItemsCommand(),
		},
	})
}

// /add — учитель отправляет фото + описание, создаётся объявление.
// Интерактивный диалог: title → description → location.
// Фото прикрепляется к команде (caption = /add) или на любом шаге.
func addCommand() wasmplugin.Trigger {
	return wasmplugin.Trigger{
		Name:        "add",
		Type:        wasmplugin.TriggerMessenger,
		Description: "Report a found item with photo",
		Nodes: []wasmplugin.Node{
			wasmplugin.NewStep("title").
				LocalizedText(cat.L("enter_title"), wasmplugin.StylePlain),

			wasmplugin.NewStep("description").
				LocalizedText(cat.L("enter_description"), wasmplugin.StylePlain),

			wasmplugin.NewStep("location").
				LocalizedText(cat.L("enter_location"), wasmplugin.StylePlain),

			// Фото — последний шаг. Пользователь отправляет картинку,
			// FileInput попадает в extractFiles() при завершении диалога.
			wasmplugin.NewStep("photo").
				LocalizedText(cat.L("send_photo"), wasmplugin.StylePlain),
		},
		Handler: handleAdd,
	}
}

func handleAdd(ctx *wasmplugin.EventContext) error {
	tr := cat.Tr(ctx.Locale())

	title := ctx.Param("title")
	description := ctx.Param("description")
	location := ctx.Param("location")

	// Сохраняем все прикреплённые фото.
	var photoIDs []string
	if ctx.HasFiles() {
		for _, f := range ctx.Files() {
			data, err := ctx.FileReadAll(f.ID)
			if err != nil {
				ctx.LogError("add: read photo: " + err.Error())
				continue
			}
			stored, err := ctx.FileStore(f.Name, f.MIMEType, f.FileType, data)
			if err != nil {
				ctx.LogError("add: store photo: " + err.Error())
				continue
			}
			photoIDs = append(photoIDs, stored.ID)
		}
	}

	db, err := openDB()
	if err != nil {
		ctx.LogError("add: db open: " + err.Error())
		ctx.Reply(wasmplugin.NewMessage(tr("error")))
		return nil
	}
	defer db.Close()

	id, err := dbInsertItem(db, lostItem{
		Title:       title,
		Description: description,
		PhotoID:     strings.Join(photoIDs, ","),
		Location:    location,
		CreatedBy:   ctx.Messenger.UserID,
	})
	if err != nil {
		ctx.LogError("add: db insert: " + err.Error())
		ctx.Reply(wasmplugin.NewMessage(tr("error")))
		return nil
	}

	ctx.Reply(wasmplugin.NewMessage(fmt.Sprintf(tr("item_saved_detail"), title, id)))
	ctx.Log(fmt.Sprintf("add: item #%d created by user %d", id, ctx.Messenger.UserID))
	return nil
}

// /list — показывает все активные объявления (текстовый список).
func listCommand() wasmplugin.Trigger {
	return wasmplugin.Trigger{
		Name:        "list",
		Type:        wasmplugin.TriggerMessenger,
		Description: "Browse all lost items",
		Handler:     handleList,
	}
}

func handleList(ctx *wasmplugin.EventContext) error {
	tr := cat.Tr(ctx.Locale())

	db, err := openDB()
	if err != nil {
		ctx.LogError("list: db: " + err.Error())
		ctx.Reply(wasmplugin.NewMessage(tr("error")))
		return nil
	}
	defer db.Close()

	items, err := dbActiveItems(db)
	if err != nil {
		ctx.LogError("list: query: " + err.Error())
		ctx.Reply(wasmplugin.NewMessage(tr("error")))
		return nil
	}

	if len(items) == 0 {
		ctx.Reply(wasmplugin.NewMessage(tr("no_items")))
		return nil
	}

	text := fmt.Sprintf(tr("items_header"), len(items)) + "\n\n"
	for _, item := range items {
		loc := item.Location
		if loc == "" {
			loc = "—"
		}
		text += fmt.Sprintf(tr("item_line"), item.ID, item.Title, loc) + "\n"
	}
	text += "\n/detail — " + tr("item_detail_header")

	ctx.Reply(wasmplugin.NewMessage(text))
	return nil
}

// /detail — выбрать предмет по номеру и посмотреть описание + фото.
func detailCommand() wasmplugin.Trigger {
	return wasmplugin.Trigger{
		Name:        "detail",
		Type:        wasmplugin.TriggerMessenger,
		Description: "View item details and photo",
		Nodes: []wasmplugin.Node{
			wasmplugin.NewStep("item_id").
				LocalizedText(cat.L("enter_item_id"), wasmplugin.StylePlain).
				Validate(`^\d+$`),
		},
		Handler: handleDetail,
	}
}

func handleDetail(ctx *wasmplugin.EventContext) error {
	tr := cat.Tr(ctx.Locale())
	rawID := ctx.Param("item_id")

	id, _ := strconv.Atoi(rawID)

	db, err := openDB()
	if err != nil {
		ctx.LogError("detail: db: " + err.Error())
		ctx.Reply(wasmplugin.NewMessage(tr("error")))
		return nil
	}
	defer db.Close()

	item, err := dbItemByID(db, id)
	if err != nil {
		ctx.LogError("detail: query: " + err.Error())
		ctx.Reply(wasmplugin.NewMessage(tr("error")))
		return nil
	}
	if item == nil {
		ctx.Reply(wasmplugin.NewMessage(fmt.Sprintf(tr("item_not_found"), rawID)))
		return nil
	}

	loc := item.Location
	if loc == "" {
		loc = "—"
	}

	header := fmt.Sprintf(tr("item_detail_header"), item.ID)
	body := fmt.Sprintf(tr("item_detail"), item.Title, item.Description, loc)
	msg := wasmplugin.NewMessage(header + "\n\n" + body)

	if item.PhotoID != "" {
		for _, pid := range strings.Split(item.PhotoID, ",") {
			if pid == "" {
				continue
			}
			meta, err := ctx.FileMeta(pid)
			if err != nil {
				ctx.LogError("detail: file meta " + pid + ": " + err.Error())
				continue
			}
			msg = msg.File(*meta, "")
		}
	}

	ctx.Reply(msg)
	return nil
}

// /myitems — показывает объявления текущего пользователя.
func myItemsCommand() wasmplugin.Trigger {
	return wasmplugin.Trigger{
		Name:        "myitems",
		Type:        wasmplugin.TriggerMessenger,
		Description: "View your postings",
		Handler:     handleMyItems,
	}
}

func handleMyItems(ctx *wasmplugin.EventContext) error {
	tr := cat.Tr(ctx.Locale())

	db, err := openDB()
	if err != nil {
		ctx.LogError("myitems: db: " + err.Error())
		ctx.Reply(wasmplugin.NewMessage(tr("error")))
		return nil
	}
	defer db.Close()

	items, err := dbItemsByUser(db, ctx.Messenger.UserID)
	if err != nil {
		ctx.LogError("myitems: query: " + err.Error())
		ctx.Reply(wasmplugin.NewMessage(tr("error")))
		return nil
	}

	if len(items) == 0 {
		ctx.Reply(wasmplugin.NewMessage(tr("no_my_items")))
		return nil
	}

	text := fmt.Sprintf(tr("my_items_header"), len(items)) + "\n\n"
	for _, item := range items {
		status := ""
		if item.Status == "resolved" {
			status = " [returned]"
		}
		text += fmt.Sprintf("#%d  %s — %s%s\n", item.ID, item.Title, item.Location, status)
	}

	ctx.Reply(wasmplugin.NewMessage(text))
	return nil
}
