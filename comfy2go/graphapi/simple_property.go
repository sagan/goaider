// @mod : new file.
// comfy2go 这个库作者真心NB，尼玛把 Property 封装了这么多层完全不知道怎么改。
// 这货以前难道是写 Java 的？
// 原来的代码改不了，自己弄一个简单点的 Property。
package graphapi

import (
	"fmt"
)

// string 类型的 property
type SimpleStringProperty struct {
	Alias         string
	Value         string
	Node          *GraphNode
	IndexValue    int
	NameValue     string
	OptionalValue bool
}

func NewSimpleStringProperty(name string, value any) *SimpleStringProperty {
	return &SimpleStringProperty{
		NameValue: name,
		Value:     fmt.Sprint(value),
	}
}

// AttachSecondaryProperty implements Property.
func (f *SimpleStringProperty) AttachSecondaryProperty(p Property) {
}

// GetAlias implements Property.
func (f *SimpleStringProperty) GetAlias() string {
	return f.Alias
}

// GetTargetNode implements Property.
func (f *SimpleStringProperty) GetTargetNode() *GraphNode {
	return f.Node
}

// GetTargetWidget implements Property.
func (f *SimpleStringProperty) GetTargetWidget() int {
	return 0
}

// GetValue implements Property.
func (f *SimpleStringProperty) GetValue() any {
	return f.Value
}

// Index implements Property.
func (f *SimpleStringProperty) Index() int {
	return f.IndexValue
}

// Name implements Property.
func (f *SimpleStringProperty) Name() string {
	return f.NameValue
}

// Optional implements Property.
func (f *SimpleStringProperty) Optional() bool {
	return f.OptionalValue
}

// Serializable implements Property.
func (f *SimpleStringProperty) Serializable() bool {
	return true
}

// SetAlias implements Property.
func (f *SimpleStringProperty) SetAlias(s string) {
	f.Alias = s
}

// SetDirectValue implements Property.
func (f *SimpleStringProperty) SetDirectValue(v *any) {
}

// SetIndex implements Property.
func (f *SimpleStringProperty) SetIndex(index int) {
	f.IndexValue = index
}

// SetSerializable implements Property.
func (f *SimpleStringProperty) SetSerializable(bool) {
}

// SetTargetWidget implements Property.
func (f *SimpleStringProperty) SetTargetWidget(node *GraphNode, index int) {
}

// SetValue implements Property.
func (f *SimpleStringProperty) SetValue(v any) error {
	f.Value = fmt.Sprint(v)
	return nil
}

// Settable implements Property.
func (f *SimpleStringProperty) Settable() bool {
	return false
}

// TargetIndex implements Property.
func (f *SimpleStringProperty) TargetIndex() int {
	return f.IndexValue
}

// ToBoolProperty implements Property.
func (f *SimpleStringProperty) ToBoolProperty() (*BoolProperty, bool) {
	return nil, false
}

// ToCascadeProperty implements Property.
func (f *SimpleStringProperty) ToCascadeProperty() (*CascadingProperty, bool) {
	return nil, false
}

// ToComboProperty implements Property.
func (f *SimpleStringProperty) ToComboProperty() (*ComboProperty, bool) {
	return nil, false
}

// ToFloatProperty implements Property.
func (f *SimpleStringProperty) ToFloatProperty() (*FloatProperty, bool) {
	return nil, false
}

// ToImageUploadProperty implements Property.
func (f *SimpleStringProperty) ToImageUploadProperty() (*ImageUploadProperty, bool) {
	return nil, false
}

// ToIntProperty implements Property.
func (f *SimpleStringProperty) ToIntProperty() (*IntProperty, bool) {
	return nil, false
}

// ToStringProperty implements Property.
func (f *SimpleStringProperty) ToStringProperty() (*StringProperty, bool) {
	return nil, false
}

// ToUnknownProperty implements Property.
func (f *SimpleStringProperty) ToUnknownProperty() (*UnknownProperty, bool) {
	return nil, false
}

// TypeString implements Property.
func (f *SimpleStringProperty) TypeString() string {
	return "string"
}

// UpdateParent implements Property.
func (f *SimpleStringProperty) UpdateParent(parent Property) {
}

// valueFromString implements Property.
func (f *SimpleStringProperty) valueFromString(value string) any {
	return nil
}

var _ Property = (*SimpleStringProperty)(nil)
