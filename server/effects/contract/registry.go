package contract

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

// Payload is implemented by all lifecycle payload structs that can be registered in the
// effect contract registry. Implementations embed ContractPayload to satisfy the interface.
type Payload interface {
	payloadMarker()
}

// ContractPayload is embedded into payload structs to mark them as contract payloads.
type ContractPayload struct{}

func (ContractPayload) payloadMarker() {}

type payloadSentinel struct{}

func (payloadSentinel) payloadMarker() {}

// NoPayload indicates that a lifecycle phase does not carry any payload.
var NoPayload Payload = payloadSentinel{}

var (
	errEmptyDefinitionID = errors.New("definition id must not be empty")
	errNilPayload        = errors.New("payload must not be nil")
	errNonPointer        = errors.New("payload must be a pointer")
	errNonStructPointer  = errors.New("payload must point to a struct")
	errInvalidOwner      = errors.New("definition owner must be LifecycleOwnerServer or LifecycleOwnerClient")
)

// LifecycleOwner specifies which runtime controls an effect instance after it
// has spawned.
type LifecycleOwner int

const (
	// LifecycleOwnerServer indicates the server manages the full lifecycle.
	LifecycleOwnerServer LifecycleOwner = iota
	// LifecycleOwnerClient hands lifecycle management to the client once the
	// spawn payload has been delivered.
	LifecycleOwnerClient
)

// Definition associates an effect contract ID with the payload types emitted during
// spawn, update, and end lifecycle phases. Ownership defaults to
// LifecycleOwnerServer unless explicitly set.
type Definition struct {
	ID     string
	Spawn  Payload
	Update Payload
	End    Payload
	Owner  LifecycleOwner
}

// Registry is a collection of effect contract definitions. Callers should Validate before use.
type Registry []Definition

// Validate ensures the registry contains unique IDs and structurally valid payload declarations.
func (r Registry) Validate() error {
	seen := make(map[string]struct{}, len(r))
	for _, def := range r {
		if err := def.validate(); err != nil {
			return fmt.Errorf("contract: %w", err)
		}
		if _, exists := seen[def.ID]; exists {
			return fmt.Errorf("contract: duplicate definition id %q", def.ID)
		}
		seen[def.ID] = struct{}{}
	}
	return nil
}

func (d Definition) validate() error {
	if strings.TrimSpace(d.ID) == "" {
		return errEmptyDefinitionID
	}
	if d.Owner < LifecycleOwnerServer || d.Owner > LifecycleOwnerClient {
		return errInvalidOwner
	}
	if err := validatePayload(d.Spawn); err != nil {
		return fmt.Errorf("spawn: %w", err)
	}
	if err := validatePayload(d.Update); err != nil {
		return fmt.Errorf("update: %w", err)
	}
	if err := validatePayload(d.End); err != nil {
		return fmt.Errorf("end: %w", err)
	}
	return nil
}

func validatePayload(payload Payload) error {
	if payload == nil {
		return errNilPayload
	}
	if payload == NoPayload {
		return nil
	}
	t := reflect.TypeOf(payload)
	if t.Kind() != reflect.Ptr {
		return fmt.Errorf("%w (%s)", errNonPointer, t)
	}
	if t.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("%w (%s)", errNonStructPointer, t)
	}
	return nil
}

// Index materialises a lookup map from the registry after validation.
func (r Registry) Index() (map[string]Definition, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	out := make(map[string]Definition, len(r))
	for _, def := range r {
		out[def.ID] = def
	}
	return out, nil
}

// MustIndex materialises the registry and panics if validation fails. Useful for tests.
func (r Registry) MustIndex() map[string]Definition {
	index, err := r.Index()
	if err != nil {
		panic(err)
	}
	return index
}
