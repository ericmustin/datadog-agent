// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-2020 Datadog, Inc.

package remote

import (
	"github.com/DataDog/datadog-agent/pkg/tagger/collectors"
)

type tagStore struct {
	store map[string]collectors.Entity
}

func newTagStore() *tagStore {
	return &tagStore{
		store: make(map[string]collectors.Entity),
	}
}

func (s *tagStore) processEvent(event collectors.EntityEvent) error {
	switch event.EventType {
	case collectors.EventTypeAdded, collectors.EventTypeModified:
		s.store[event.Entity.ID] = event.Entity
	case collectors.EventTypeDeleted:
		delete(s.store, event.Entity.ID)
	}

	return nil
}

func (s *tagStore) getEntity(entityID string) (collectors.Entity, error) {
	return s.store[entityID], nil
}

func (s *tagStore) listEntities() []collectors.Entity {
	entities := make([]collectors.Entity, 0, len(s.store))

	for _, e := range s.store {
		entities = append(entities, e)
	}

	return entities
}
