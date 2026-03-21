package main

import (
	"context"
	"sync"

	"SuperBotGo/internal/model"
	pluginLink "SuperBotGo/internal/plugin/link"
	"SuperBotGo/internal/plugin/project"
	"SuperBotGo/internal/state/storage"
)

type inMemoryDialogStorage struct {
	mu    sync.RWMutex
	store map[model.GlobalUserID]*model.DialogState
}

func (s *inMemoryDialogStorage) Save(_ context.Context, userID model.GlobalUserID, ds model.DialogState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.store == nil {
		s.store = make(map[model.GlobalUserID]*model.DialogState)
	}
	copy := ds
	s.store[userID] = &copy
	return nil
}

func (s *inMemoryDialogStorage) Load(_ context.Context, userID model.GlobalUserID) (*model.DialogState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.store == nil {
		return nil, nil
	}
	ds, ok := s.store[userID]
	if !ok {
		return nil, nil
	}
	copy := *ds
	return &copy, nil
}

func (s *inMemoryDialogStorage) Delete(_ context.Context, userID model.GlobalUserID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.store != nil {
		delete(s.store, userID)
	}
	return nil
}

var _ storage.DialogStorage = (*inMemoryDialogStorage)(nil)

type placeholderProjectStore struct{}

func (s *placeholderProjectStore) ListProjects() ([]model.Project, error) {
	return nil, nil
}

func (s *placeholderProjectStore) FindProject(_ context.Context, _ int64) (*model.Project, error) {
	return nil, nil
}

func (s *placeholderProjectStore) SaveProject(_ context.Context, name, description string) (*model.Project, error) {
	return &model.Project{ID: 1, Name: name, Description: description}, nil
}

var _ project.ProjectStore = (*placeholderProjectStore)(nil)

type placeholderChatStore struct{}

func (s *placeholderChatStore) ListChats() ([]model.ChatReference, error) {
	return nil, nil
}

func (s *placeholderChatStore) FindChat(_ context.Context, _ model.ChannelType, _ string) (*model.ChatReference, error) {
	return nil, nil
}

func (s *placeholderChatStore) FindChatByID(_ context.Context, _ int64) (*model.ChatReference, error) {
	return nil, nil
}

func (s *placeholderChatStore) RegisterChat(_ context.Context, ref model.ChatReference) (*model.ChatReference, error) {
	ref.ID = 1
	return &ref, nil
}

func (s *placeholderChatStore) BindChat(_ context.Context, _, _ int64) error {
	return nil
}

var _ project.ChatStore = (*placeholderChatStore)(nil)

type placeholderAccountLinker struct{}

func (l *placeholderAccountLinker) InitiateLinking(_ context.Context, _ model.GlobalUserID) pluginLink.LinkResult {
	return pluginLink.LinkResult{
		Kind:    pluginLink.LinkCodeGenerated,
		Code:    "PLACEHOLDER-CODE",
		Message: "",
	}
}

func (l *placeholderAccountLinker) CompleteLinking(_ context.Context, _ model.GlobalUserID, _ string) pluginLink.LinkResult {
	return pluginLink.LinkResult{
		Kind:    pluginLink.LinkError,
		Message: "Account linking not yet implemented",
	}
}

var _ pluginLink.AccountLinker = (*placeholderAccountLinker)(nil)
