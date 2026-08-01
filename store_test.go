package main

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestOpenMissingFileIsEmpty(t *testing.T) {
	s, err := OpenStore(filepath.Join(t.TempDir(), "anime.csv"))
	if err != nil {
		t.Fatalf("OpenStore() error = %v", err)
	}
	if got := s.Entries(nil); len(got) != 0 {
		t.Errorf("Entries() = %v, want empty", got)
	}
}

func TestAddFindUpdateDelete(t *testing.T) {
	s, err := OpenStore(filepath.Join(t.TempDir(), "anime.csv"))
	if err != nil {
		t.Fatalf("OpenStore() error = %v", err)
	}

	if _, err := s.Add(Anime{Title: "Cowboy Bebop"}); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if _, err := s.Add(Anime{Title: "cowboy bebop"}); err == nil {
		t.Error("Add() with duplicate title (case-insensitive) should fail")
	}
	if _, err := s.Add(Anime{}); !errors.Is(err, ErrEmptyTitle) {
		t.Errorf("Add() with empty title error = %v, want %v", err, ErrEmptyTitle)
	}

	found, err := s.FindByTitle("COWBOY BEBOP")
	if err != nil {
		t.Fatalf("FindByTitle() error = %v", err)
	}
	if found.Title != "Cowboy Bebop" {
		t.Errorf("FindByTitle() = %q, want %q", found.Title, "Cowboy Bebop")
	}

	updated, err := s.Update("Cowboy Bebop", Anime{Title: "Cowboy Bebop", Progress: 26})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Progress != 26 {
		t.Errorf("Update() progress = %d, want 26", updated.Progress)
	}

	if _, err := s.Update("no such title", Anime{Title: "x"}); !errors.Is(err, ErrTitleNotFound) {
		t.Errorf("Update() missing title error = %v, want %v", err, ErrTitleNotFound)
	}

	if err := s.Delete("Cowboy Bebop"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := s.FindByTitle("Cowboy Bebop"); !errors.Is(err, ErrTitleNotFound) {
		t.Errorf("FindByTitle() after delete error = %v, want %v", err, ErrTitleNotFound)
	}
}

func TestEntriesFilterByStatus(t *testing.T) {
	s, err := OpenStore(filepath.Join(t.TempDir(), "anime.csv"), WithDefaultStatus(StatusWatching))
	if err != nil {
		t.Fatalf("OpenStore() error = %v", err)
	}
	if _, err := s.Add(Anime{Title: "A", Status: StatusWatching}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Add(Anime{Title: "B", Status: StatusCompleted}); err != nil {
		t.Fatal(err)
	}

	watching := StatusWatching
	got := s.Entries(&watching)
	if len(got) != 1 || got[0].Title != "A" {
		t.Errorf("Entries(watching) = %v, want [A]", got)
	}

	if got := s.Entries(nil); len(got) != 2 {
		t.Errorf("Entries(nil) = %v, want 2 entries", got)
	}
}

func TestSaveReloadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "anime.csv")
	s, err := OpenStore(path)
	if err != nil {
		t.Fatalf("OpenStore() error = %v", err)
	}

	started := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	rating := float32(9.5)
	want := Anime{
		Title:        "Cowboy Bebop",
		Progress:     26,
		Status:       StatusRewatching,
		StartedAt:    &started,
		Rating:       &rating,
		TotalRewatch: 1,
		Notes:        "great show",
	}
	if _, err := s.Add(want); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	reloaded, err := OpenStore(path)
	if err != nil {
		t.Fatalf("reopen OpenStore() error = %v", err)
	}
	got, err := reloaded.FindByTitle("Cowboy Bebop")
	if err != nil {
		t.Fatalf("FindByTitle() error = %v", err)
	}

	if got.Title != want.Title || got.Progress != want.Progress || got.Status != want.Status ||
		got.TotalRewatch != want.TotalRewatch || got.Notes != want.Notes {
		t.Errorf("reloaded entry = %+v, want %+v", got, want)
	}
	if got.StartedAt == nil || !got.StartedAt.Equal(*want.StartedAt) {
		t.Errorf("reloaded StartedAt = %v, want %v", got.StartedAt, want.StartedAt)
	}
	if got.Rating == nil || *got.Rating != *want.Rating {
		t.Errorf("reloaded Rating = %v, want %v", got.Rating, want.Rating)
	}
}
