// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package testutils

import (
	"net/http"
	"testing"
)

func TestSetResponseSequence_AdvancesThenRepeatsLast(t *testing.T) {
	m := NewMockGitHubServer()
	defer m.Close()
	const path = "/repos/o/r/actions/runs/1"
	m.SetResponseSequence(path, []MockResponse{
		{StatusCode: http.StatusOK, Body: map[string]interface{}{"status": "waiting"}},
		{StatusCode: http.StatusOK, Body: map[string]interface{}{"status": "waiting", "step": 2}},
		{StatusCode: http.StatusOK, Body: map[string]interface{}{"status": "completed"}},
	})

	first, ok := m.nextResponse(path)
	if !ok || first.Body.(map[string]interface{})["status"] != "waiting" {
		t.Fatalf("1st call = %+v, want the first sequence entry", first)
	}
	second, _ := m.nextResponse(path)
	if second.Body.(map[string]interface{})["step"] != 2 {
		t.Fatalf("2nd call = %+v, want the second sequence entry", second)
	}
	third, _ := m.nextResponse(path)
	if third.Body.(map[string]interface{})["status"] != "completed" {
		t.Fatalf("3rd call = %+v, want the third sequence entry", third)
	}
	// Exhausted: repeats the last entry indefinitely, not an empty/missing response.
	fourth, ok := m.nextResponse(path)
	if !ok || fourth.Body.(map[string]interface{})["status"] != "completed" {
		t.Fatalf("4th call (post-exhaustion) = %+v, ok=%v, want the last entry repeated", fourth, ok)
	}
	fifth, _ := m.nextResponse(path)
	if fifth.Body.(map[string]interface{})["status"] != "completed" {
		t.Fatalf("5th call = %+v, want still repeating the last entry", fifth)
	}
}

func TestSetResponseSequence_TakesPriorityOverSetResponse(t *testing.T) {
	m := NewMockGitHubServer()
	defer m.Close()
	const path = "/repos/o/r/actions/runs/1"
	m.SetResponse(path, MockResponse{StatusCode: http.StatusOK, Body: map[string]interface{}{"from": "plain"}})
	m.SetResponseSequence(path, []MockResponse{
		{StatusCode: http.StatusOK, Body: map[string]interface{}{"from": "sequence"}},
	})

	resp, ok := m.nextResponse(path)
	if !ok || resp.Body.(map[string]interface{})["from"] != "sequence" {
		t.Fatalf("nextResponse() = %+v, want the sequence entry to win", resp)
	}
}

func TestSetResponseSequence_NoSequenceFallsBackToPlainResponse(t *testing.T) {
	m := NewMockGitHubServer()
	defer m.Close()
	const path = "/repos/o/r/actions/runs/1"
	m.SetResponse(path, MockResponse{StatusCode: http.StatusOK, Body: map[string]interface{}{"from": "plain"}})

	resp, ok := m.nextResponse(path)
	if !ok || resp.Body.(map[string]interface{})["from"] != "plain" {
		t.Fatalf("nextResponse() = %+v, want the plain response", resp)
	}
}

func TestSetResponseSequence_NeitherConfiguredNoOverride(t *testing.T) {
	m := NewMockGitHubServer()
	defer m.Close()

	if _, ok := m.nextResponse("/never/configured"); ok {
		t.Error("expected no override for an unconfigured path")
	}
}
