package main

import (
	"testing"
)

func TestCamelToSnake(t *testing.T) {
	input := "TestFunction"
	expected := "test_function"
	result := camelToSnake(input)
	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}
