package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStoreParsesExampleCSV(t *testing.T) {
	store, err := NewStore("anime-progress-example.csv")
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	entries := store.Entries()
	if len(entries) != 5 {
		t.Fatalf("len(entries) = %d, want 5", len(entries))
	}
	if entries[0].Title != "Attack on Titan" {
		t.Fatalf("first title = %q", entries[0].Title)
	}
	if entries[2].Status != StatusWatching {
		t.Fatalf("third status = %q", entries[2].Status)
	}
	if entries[3].Status != StatusPlanToWatch {
		t.Fatalf("fourth status = %q", entries[3].Status)
	}
}

func TestStoreParsesLegacyProgressFile(t *testing.T) {
	entries, structured, err := parseAnimeFile([]byte("9.8 [*] Attack on titan\n[x] k-on (1 серия)\n"))
	if err != nil {
		t.Fatalf("parseAnimeFile() error = %v", err)
	}
	if structured {
		t.Fatal("structured = true, want false")
	}
	if len(entries) == 0 {
		t.Fatal("len(entries) = 0, want legacy entries")
	}
	if entries[0].Title == "" {
		t.Fatal("first legacy title is empty")
	}
	if entries[0].Status != StatusCompleted {
		t.Fatalf("first legacy status = %q, want completed", entries[0].Status)
	}
}

func TestStoreMutationsWriteStructuredCSV(t *testing.T) {
	path := filepath.Join(t.TempDir(), "anime-progress.csv")
	store, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	entry, err := store.Add(Anime{
		Title:    "Mob Psycho 100",
		Status:   StatusWatching,
		Progress: 3,
	})
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if _, err := store.AdjustProgress(entry.ID, 1); err != nil {
		t.Fatalf("AdjustProgress() error = %v", err)
	}

	reloaded, err := NewStore(path)
	if err != nil {
		t.Fatalf("reload NewStore() error = %v", err)
	}
	entries := reloaded.Entries()
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}
	if entries[0].Progress != 4 {
		t.Fatalf("progress = %d, want 4", entries[0].Progress)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(content[:3]) != "id," {
		t.Fatalf("file does not start with structured header: %q", string(content))
	}
}

func TestStoreUpdatePreservesProvidedMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "anime-progress.csv")
	store, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	start := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	entry, err := store.Add(Anime{
		Title:        "Haibane Renmei",
		Status:       StatusCompleted,
		StartDate:    start,
		TotalRewatch: 2,
	})
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	entry.Title = "Haibane Renmei Updated"
	updated, err := store.Update(entry.ID, entry)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if !updated.StartDate.Equal(start) {
		t.Fatalf("StartDate = %v, want %v", updated.StartDate, start)
	}
	if updated.TotalRewatch != 2 {
		t.Fatalf("TotalRewatch = %d, want 2", updated.TotalRewatch)
	}
}
