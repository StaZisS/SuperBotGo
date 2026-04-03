package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/superbot/wasmplugin"
)

// KV key: "photos:<userID>" → comma-separated file IDs.
func photosKey(userID int64) string {
	return "photos:" + strconv.FormatInt(userID, 10)
}

func photoCommand() wasmplugin.Trigger {
	return wasmplugin.Trigger{
		Name:        "photo",
		Type:        wasmplugin.TriggerMessenger,
		Description: "Save a photo to your gallery",
		Nodes:       nil,
		Handler:     handlePhoto,
	}
}

func handlePhoto(ctx *wasmplugin.EventContext) error {
	tr := cat.Tr(ctx.Locale())

	if !ctx.HasFiles() {
		ctx.Reply(wasmplugin.NewMessage(tr("photo_send_prompt")))
		return nil
	}

	files := ctx.Files()
	userID := ctx.Messenger.UserID
	ctx.Log(fmt.Sprintf("photo: user %d sent %d file(s)", userID, len(files)))

	// Load existing list of saved file IDs.
	key := photosKey(userID)
	existing, _, _ := ctx.KVGet(key)

	var ids []string
	if existing != "" {
		ids = strings.Split(existing, ",")
	}

	saved := 0
	for _, f := range files {
		data, err := ctx.FileReadAll(f.ID)
		if err != nil {
			ctx.LogError("photo: read " + f.ID + ": " + err.Error())
			continue
		}

		stored, err := ctx.FileStore(f.Name, f.MIMEType, f.FileType, data)
		if err != nil {
			ctx.LogError("photo: store: " + err.Error())
			continue
		}

		ids = append(ids, stored.ID)
		saved++
		ctx.Log(fmt.Sprintf("photo: stored %s (%d bytes) → %s", f.Name, len(data), stored.ID))
	}

	if saved > 0 {
		// Persist updated list.
		if err := ctx.KVSet(key, strings.Join(ids, ",")); err != nil {
			ctx.LogError("photo: kv set: " + err.Error())
		}
		ctx.Reply(wasmplugin.NewMessage(fmt.Sprintf("%s (%d). %s: %d",
			tr("photo_saved"), saved, tr("gallery_total"), len(ids))))
	} else {
		ctx.Reply(wasmplugin.NewMessage(tr("photo_error")))
	}

	return nil
}

func galleryCommand() wasmplugin.Trigger {
	return wasmplugin.Trigger{
		Name:        "gallery",
		Type:        wasmplugin.TriggerMessenger,
		Description: "View your saved photos",
		Nodes:       nil,
		Handler:     handleGallery,
	}
}

func handleGallery(ctx *wasmplugin.EventContext) error {
	tr := cat.Tr(ctx.Locale())
	userID := ctx.Messenger.UserID

	val, found, err := ctx.KVGet(photosKey(userID))
	if err != nil {
		ctx.LogError("gallery: kv get: " + err.Error())
		ctx.Reply(wasmplugin.NewMessage(tr("error")))
		return nil
	}

	if !found || val == "" {
		ctx.Reply(wasmplugin.NewMessage(tr("gallery_empty")))
		return nil
	}

	ids := strings.Split(val, ",")
	msg := wasmplugin.NewMessage(fmt.Sprintf("%s: %d", tr("gallery_total"), len(ids)))

	for _, id := range ids {
		meta, err := ctx.FileMeta(id)
		if err != nil {
			ctx.LogError("gallery: file meta " + id + ": " + err.Error())
			continue
		}
		msg = msg.File(*meta, "")
	}

	ctx.Reply(msg)
	return nil
}
