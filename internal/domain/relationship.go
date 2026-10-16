package domain

import (
	"errors"
	"strings"
)

var (
	ErrInvalidRelationship = errors.New("parent-child relationship is invalid")
	ErrSelfRelationship    = errors.New("a person cannot be their own parent")
)

type ParentChild struct {
	ParentID string
	ChildID  string
}

func (r ParentChild) Validate() error {
	if strings.TrimSpace(r.ParentID) == "" || strings.TrimSpace(r.ChildID) == "" {
		return ErrInvalidRelationship
	}
	if r.ParentID == r.ChildID {
		return ErrSelfRelationship
	}
	return nil
}

type GraphSnapshot struct {
	People      []Person
	ParentChild []ParentChild
}
