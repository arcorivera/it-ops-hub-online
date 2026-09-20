package response

import (
    "encoding/json"
    "reflect"
    "testing"
)

type nestedPayload struct {
    Items []string `json:"items"`
}

type testPayload struct {
    Items []string `json:"items"`
    Nested nestedPayload `json:"nested"`
}

func TestNormalizedConvertsNilSlicesToEmptyArrays(t *testing.T) {
    got := normalized(testPayload{Nested: nestedPayload{}})
    payload, ok := got.(testPayload)
    if !ok {
        t.Fatalf("normalized returned %T, want testPayload", got)
    }
    if payload.Items == nil || payload.Nested.Items == nil {
        t.Fatal("expected nil slices to become non-nil empty slices")
    }
    encoded, err := json.Marshal(Envelope{Data: got})
    if err != nil {
        t.Fatal(err)
    }
    if string(encoded) == `{"data":{"items":null,"nested":{"items":null}}}` {
        t.Fatal("normalized payload still encoded nil slices as null")
    }
    if !reflect.DeepEqual(payload.Items, []string{}) || !reflect.DeepEqual(payload.Nested.Items, []string{}) {
        t.Fatal("expected empty slices")
    }
}
