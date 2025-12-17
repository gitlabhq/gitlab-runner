package common

import (
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"reflect"
	"slices"

	"gitlab.com/gitlab-org/moa"
	"gitlab.com/gitlab-org/step-runner/pkg/api/expression"

	"gitlab.com/gitlab-org/moa/value"
)

type JobInputs struct {
	inputs    []expression.Input
	evaluator *expression.Evaluator
}

type JobInput struct {
	Key   string        `json:"key"`
	Value JobInputValue `json:"value"`
}

type JobInputValue struct {
	Type      JobInputContentTypeName `json:"type"`
	Content   value.Value             `json:"content"`
	Sensitive bool                    `json:"sensitive"`
}

type JobInputContentTypeName string

type InputExpander interface {
	Expand(*JobInputs) error
}

type InputInterpolationError struct {
	err error
}

func (e *InputInterpolationError) Error() string {
	return fmt.Sprintf("failed to interpolate job inputs: %s", e.err.Error())
}

const (
	JobInputContentTypeNameString  JobInputContentTypeName = "string"
	JobInputContentTypeNameNumber  JobInputContentTypeName = "number"
	JobInputContentTypeNameBoolean JobInputContentTypeName = "boolean"
	JobInputContentTypeNameArray   JobInputContentTypeName = "array"
	JobInputContentTypeNameStruct  JobInputContentTypeName = "struct"
)

var (
	errInputExpanderNotSupported = errors.New("type does not implement InputExpander")
)

func (t JobInputContentTypeName) MoaKind() (value.Kind, error) {
	switch t {
	case JobInputContentTypeNameString:
		return value.StringKind, nil
	case JobInputContentTypeNameNumber:
		return value.NumberKind, nil
	case JobInputContentTypeNameBoolean:
		return value.BoolKind, nil
	case JobInputContentTypeNameArray:
		return value.ArrayKind, nil
	case JobInputContentTypeNameStruct:
		return value.ObjectKind, nil
	default:
		return value.NullKind, errors.New("type is unknown")
	}
}

var (
	ErrSensitiveUnsupported = errors.New("sensitive inputs are unsupported in interpolations yet")
)

func (i *JobInput) UnmarshalJSON(data []byte) error {
	type alias JobInput
	if err := json.Unmarshal(data, (*alias)(i)); err != nil {
		return err
	}

	if err := i.validate(); err != nil {
		return err
	}

	return nil
}

func (i *JobInput) validate() error {
	// verify that input has key
	if i.Key == "" {
		return fmt.Errorf("input without key")
	}

	if i.Value.Content.Kind() == value.NullKind {
		return fmt.Errorf("input %q is null, must have valid value", i.Key)
	}

	// verify that we have a supported and valid input and moa type
	moaKind, err := i.Value.Type.MoaKind()
	if err != nil {
		return fmt.Errorf("invalid type %q for input %q: %w", i.Value.Type, i.Key, err)
	}

	// verify that the input content actually has the announced type
	if moaKind != i.Value.Content.Kind() {
		return fmt.Errorf("mismatching type of input %q. Announced %q, but got %q", i.Key, moaKind, i.Value.Content.Kind())
	}

	return nil
}

func (i *JobInputs) UnmarshalJSON(data []byte) error {
	var inputs []JobInput

	if err := json.Unmarshal(data, &inputs); err != nil {
		return err
	}

	jobInputs, err := newJobInputs(inputs)
	if err != nil {
		return err
	}
	*i = jobInputs

	return nil
}

func newJobInputs(inputs []JobInput) (JobInputs, error) {
	i := JobInputs{}

	for _, input := range inputs {
		// post-process sensitive mark for input value
		v := input.Value.Content
		if input.Value.Sensitive {
			v = v.WithMarks(expression.Sensitive)
		}

		i.inputs = append(i.inputs, expression.Input{
			Key:   input.Key,
			Value: v,
		})
	}

	e, err := expression.NewEvaluator(value.Object(&i))
	if err != nil {
		return JobInputs{}, err
	}
	i.evaluator = e

	return i, nil
}

func (i *JobInputs) All() iter.Seq2[value.Value, value.Value] {
	return func(yield func(value.Value, value.Value) bool) {
		for _, input := range i.inputs {
			if !yield(value.String(input.Key), input.Value) {
				return
			}
		}
	}
}

func (i *JobInputs) Attr(a string) (value.Value, error) {
	idx := slices.IndexFunc(i.inputs, func(x expression.Input) bool {
		return x.Key == a
	})
	if idx < 0 {
		return value.Null(), value.ErrAttributeNotFound
	}
	return i.inputs[idx].Value, nil
}

func (i *JobInputs) Get(key value.Value) (value.Value, error) {
	if key.Kind() != value.StringKind {
		return value.Null(), fmt.Errorf("%w: object requires string key not %v", value.ErrInvalidKey, key.Kind())
	}

	return i.Attr(key.String())
}

func (i *JobInputs) Keys() iter.Seq[value.Value] {
	return func(yield func(value.Value) bool) {
		for _, v := range i.inputs {
			if !yield(value.String(v.Key)) {
				return
			}
		}
	}
}

func (i *JobInputs) Len() int {
	return len(i.inputs)
}

func (i *JobInputs) Values() iter.Seq[value.Value] {
	return func(yield func(value.Value) bool) {
		for _, v := range i.inputs {
			if !yield(v.Value) {
				return
			}
		}
	}
}

func (i *JobInputs) WithMarks(marks uint16) value.Mapper {
	// FIXME: what should we do here ...
	return i
}

func (i *JobInputs) Expand(text string) (string, error) {
	if i == nil || i.evaluator == nil {
		return text, nil
	}

	expr, err := moa.ParseTemplate(text)
	if err != nil {
		return "", &InputInterpolationError{err: err}
	}

	result, err := i.evaluator.Eval(text, expr)
	if err != nil {
		return "", &InputInterpolationError{err: err}
	}

	if result.HasMarks(expression.Sensitive) {
		return "", ErrSensitiveUnsupported
	}

	return result.String(), nil
}

func ExpandInputs(inputs *JobInputs, v any) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return fmt.Errorf("expected struct, got %s", rv.Kind())
	}

	err := processStruct(inputs, rv)
	if err != nil {
		e := &InputInterpolationError{}
		if errors.As(err, &e) {
			return e
		}

		return err
	}

	return nil
}

//nolint:gocognit
func processStruct(inputs *JobInputs, rv reflect.Value) error {
	err := tryExpanderInterface(inputs, rv)
	switch {
	case errors.Is(err, errInputExpanderNotSupported):
	case err != nil:
		return err
	default:
		// Successfully expanded using the interface
		return nil
	}

	rt := rv.Type()
	for i := 0; i < rv.NumField(); i++ {
		field := rv.Field(i)
		if !field.CanInterface() {
			continue
		}

		fieldType := rt.Field(i)
		inputsTag := fieldType.Tag.Get("inputs")
		if inputsTag != "expand" {
			continue
		}

		err := tryExpanderInterface(inputs, field)
		switch {
		case errors.Is(err, errInputExpanderNotSupported):
		case err != nil:
			return err
		default:
			// Successfully expanded using the interface
			continue
		}

		switch field.Kind() {
		case reflect.String:
			if err := expandStringField(inputs, field); err != nil {
				return fmt.Errorf("failed to expand string field %s: %w", fieldType.Name, err)
			}
		case reflect.Struct:
			if err := processStruct(inputs, field); err != nil {
				return fmt.Errorf("failed to process struct field %s: %w", fieldType.Name, err)
			}
		case reflect.Slice:
			if err := expandSlice(inputs, field); err != nil {
				return fmt.Errorf("failed to expand slice field %s: %w", fieldType.Name, err)
			}
		default:
			return fmt.Errorf("field %s has inputs:expand tag but is neither string-based nor struct (type: %s)",
				fieldType.Name, field.Type())
		}
	}

	return nil
}

func tryExpanderInterface(inputs *JobInputs, field reflect.Value) error {
	var fieldInterface any

	// We need to get the address if possible since methods might be on pointer receiver
	if field.CanAddr() {
		fieldInterface = field.Addr().Interface()
	} else {
		fieldInterface = field.Interface()
	}

	expander, ok := fieldInterface.(InputExpander)
	if !ok {
		return errInputExpanderNotSupported
	}

	return expander.Expand(inputs)
}

// expandStringField expands a string-based field
func expandStringField(inputs *JobInputs, field reflect.Value) error {
	if !field.CanAddr() {
		return errors.New("field is not addressable")
	}

	if !field.CanSet() {
		return errors.New("field is not settable")
	}

	expandedValue, err := inputs.Expand(field.String())
	if err != nil {
		return err
	}

	field.SetString(expandedValue)
	return nil
}

func expandSlice(inputs *JobInputs, field reflect.Value) error {
	if field.Len() == 0 {
		return nil
	}

	elemType := field.Type().Elem()
	switch elemType.Kind() {
	case reflect.String:
		return expandStringSlice(inputs, field)
	case reflect.Struct:
		return expandStructSlice(inputs, field)
	default:
		return fmt.Errorf("slice elements must be either strings or structs (element type: %s)", elemType)
	}
}

func expandStringSlice(inputs *JobInputs, field reflect.Value) error {
	for i := range field.Len() {
		elem := field.Index(i)
		if err := expandStringField(inputs, elem); err != nil {
			return fmt.Errorf("failed to expand element %d: %w", i, err)
		}
	}
	return nil
}

func expandStructSlice(inputs *JobInputs, field reflect.Value) error {
	for i := range field.Len() {
		elem := field.Index(i)
		if err := processStruct(inputs, elem); err != nil {
			return fmt.Errorf("failed to process struct element %d: %w", i, err)
		}
	}
	return nil
}
