// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.6
// 	protoc        v4.24.4
// source: v1alpha1/resource.proto

// Resource package defines protobuf serialization of COSI resources.

package v1alpha1

import (
	reflect "reflect"
	sync "sync"
	unsafe "unsafe"

	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type LabelTerm_Operation int32

const (
	// Label exists.
	LabelTerm_EXISTS LabelTerm_Operation = 0
	// Label value is equal.
	LabelTerm_EQUAL LabelTerm_Operation = 1
	// Label doesn't exist.
	//
	// Deprecated: Marked as deprecated in v1alpha1/resource.proto.
	LabelTerm_NOT_EXISTS LabelTerm_Operation = 2
	// Label value is in the set.
	LabelTerm_IN LabelTerm_Operation = 3
	// Label value is less.
	LabelTerm_LT LabelTerm_Operation = 4
	// Label value is less or equal.
	LabelTerm_LTE LabelTerm_Operation = 5
	// Label value is less than number.
	LabelTerm_LT_NUMERIC LabelTerm_Operation = 6
	// Label value is less or equal numeric.
	LabelTerm_LTE_NUMERIC LabelTerm_Operation = 7
)

// Enum value maps for LabelTerm_Operation.
var (
	LabelTerm_Operation_name = map[int32]string{
		0: "EXISTS",
		1: "EQUAL",
		2: "NOT_EXISTS",
		3: "IN",
		4: "LT",
		5: "LTE",
		6: "LT_NUMERIC",
		7: "LTE_NUMERIC",
	}
	LabelTerm_Operation_value = map[string]int32{
		"EXISTS":      0,
		"EQUAL":       1,
		"NOT_EXISTS":  2,
		"IN":          3,
		"LT":          4,
		"LTE":         5,
		"LT_NUMERIC":  6,
		"LTE_NUMERIC": 7,
	}
)

func (x LabelTerm_Operation) Enum() *LabelTerm_Operation {
	p := new(LabelTerm_Operation)
	*p = x
	return p
}

func (x LabelTerm_Operation) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (LabelTerm_Operation) Descriptor() protoreflect.EnumDescriptor {
	return file_v1alpha1_resource_proto_enumTypes[0].Descriptor()
}

func (LabelTerm_Operation) Type() protoreflect.EnumType {
	return &file_v1alpha1_resource_proto_enumTypes[0]
}

func (x LabelTerm_Operation) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use LabelTerm_Operation.Descriptor instead.
func (LabelTerm_Operation) EnumDescriptor() ([]byte, []int) {
	return file_v1alpha1_resource_proto_rawDescGZIP(), []int{3, 0}
}

// Metadata represents resource metadata.
//
// (namespace, type, id) is a resource pointer.
// (version) is a current resource version.
// (owner) is filled in for controller-managed resources with controller name.
// (phase) indicates whether resource is going through tear down phase.
// (finalizers) are attached controllers blocking teardown of the resource.
// (labels) and (annotations) are free-form key-value pairs; labels allow queries.
type Metadata struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Namespace     string                 `protobuf:"bytes,1,opt,name=namespace,proto3" json:"namespace,omitempty"`
	Type          string                 `protobuf:"bytes,2,opt,name=type,proto3" json:"type,omitempty"`
	Id            string                 `protobuf:"bytes,3,opt,name=id,proto3" json:"id,omitempty"`
	Version       string                 `protobuf:"bytes,4,opt,name=version,proto3" json:"version,omitempty"`
	Owner         string                 `protobuf:"bytes,5,opt,name=owner,proto3" json:"owner,omitempty"`
	Phase         string                 `protobuf:"bytes,6,opt,name=phase,proto3" json:"phase,omitempty"`
	Created       *timestamppb.Timestamp `protobuf:"bytes,7,opt,name=created,proto3" json:"created,omitempty"`
	Updated       *timestamppb.Timestamp `protobuf:"bytes,8,opt,name=updated,proto3" json:"updated,omitempty"`
	Finalizers    []string               `protobuf:"bytes,9,rep,name=finalizers,proto3" json:"finalizers,omitempty"`
	Annotations   map[string]string      `protobuf:"bytes,11,rep,name=annotations,proto3" json:"annotations,omitempty" protobuf_key:"bytes,1,opt,name=key" protobuf_val:"bytes,2,opt,name=value"`
	Labels        map[string]string      `protobuf:"bytes,10,rep,name=labels,proto3" json:"labels,omitempty" protobuf_key:"bytes,1,opt,name=key" protobuf_val:"bytes,2,opt,name=value"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Metadata) Reset() {
	*x = Metadata{}
	mi := &file_v1alpha1_resource_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Metadata) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Metadata) ProtoMessage() {}

func (x *Metadata) ProtoReflect() protoreflect.Message {
	mi := &file_v1alpha1_resource_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Metadata.ProtoReflect.Descriptor instead.
func (*Metadata) Descriptor() ([]byte, []int) {
	return file_v1alpha1_resource_proto_rawDescGZIP(), []int{0}
}

func (x *Metadata) GetNamespace() string {
	if x != nil {
		return x.Namespace
	}
	return ""
}

func (x *Metadata) GetType() string {
	if x != nil {
		return x.Type
	}
	return ""
}

func (x *Metadata) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *Metadata) GetVersion() string {
	if x != nil {
		return x.Version
	}
	return ""
}

func (x *Metadata) GetOwner() string {
	if x != nil {
		return x.Owner
	}
	return ""
}

func (x *Metadata) GetPhase() string {
	if x != nil {
		return x.Phase
	}
	return ""
}

func (x *Metadata) GetCreated() *timestamppb.Timestamp {
	if x != nil {
		return x.Created
	}
	return nil
}

func (x *Metadata) GetUpdated() *timestamppb.Timestamp {
	if x != nil {
		return x.Updated
	}
	return nil
}

func (x *Metadata) GetFinalizers() []string {
	if x != nil {
		return x.Finalizers
	}
	return nil
}

func (x *Metadata) GetAnnotations() map[string]string {
	if x != nil {
		return x.Annotations
	}
	return nil
}

func (x *Metadata) GetLabels() map[string]string {
	if x != nil {
		return x.Labels
	}
	return nil
}

// Spec defines content of the resource.
type Spec struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Protobuf-serialized representation of the resource.
	ProtoSpec []byte `protobuf:"bytes,1,opt,name=proto_spec,json=protoSpec,proto3" json:"proto_spec,omitempty"`
	// YAML representation of the spec (optional).
	YamlSpec      string `protobuf:"bytes,2,opt,name=yaml_spec,json=yamlSpec,proto3" json:"yaml_spec,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Spec) Reset() {
	*x = Spec{}
	mi := &file_v1alpha1_resource_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Spec) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Spec) ProtoMessage() {}

func (x *Spec) ProtoReflect() protoreflect.Message {
	mi := &file_v1alpha1_resource_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Spec.ProtoReflect.Descriptor instead.
func (*Spec) Descriptor() ([]byte, []int) {
	return file_v1alpha1_resource_proto_rawDescGZIP(), []int{1}
}

func (x *Spec) GetProtoSpec() []byte {
	if x != nil {
		return x.ProtoSpec
	}
	return nil
}

func (x *Spec) GetYamlSpec() string {
	if x != nil {
		return x.YamlSpec
	}
	return ""
}

// Resource is a combination of metadata and spec.
type Resource struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Metadata      *Metadata              `protobuf:"bytes,1,opt,name=metadata,proto3" json:"metadata,omitempty"`
	Spec          *Spec                  `protobuf:"bytes,2,opt,name=spec,proto3" json:"spec,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Resource) Reset() {
	*x = Resource{}
	mi := &file_v1alpha1_resource_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Resource) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Resource) ProtoMessage() {}

func (x *Resource) ProtoReflect() protoreflect.Message {
	mi := &file_v1alpha1_resource_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Resource.ProtoReflect.Descriptor instead.
func (*Resource) Descriptor() ([]byte, []int) {
	return file_v1alpha1_resource_proto_rawDescGZIP(), []int{2}
}

func (x *Resource) GetMetadata() *Metadata {
	if x != nil {
		return x.Metadata
	}
	return nil
}

func (x *Resource) GetSpec() *Spec {
	if x != nil {
		return x.Spec
	}
	return nil
}

// LabelTerm is an expression on a label.
type LabelTerm struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	Key   string                 `protobuf:"bytes,1,opt,name=key,proto3" json:"key,omitempty"`
	Op    LabelTerm_Operation    `protobuf:"varint,2,opt,name=op,proto3,enum=cosi.resource.LabelTerm_Operation" json:"op,omitempty"`
	Value []string               `protobuf:"bytes,3,rep,name=value,proto3" json:"value,omitempty"`
	// Inverts the condition.
	Invert        bool `protobuf:"varint,5,opt,name=invert,proto3" json:"invert,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *LabelTerm) Reset() {
	*x = LabelTerm{}
	mi := &file_v1alpha1_resource_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *LabelTerm) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*LabelTerm) ProtoMessage() {}

func (x *LabelTerm) ProtoReflect() protoreflect.Message {
	mi := &file_v1alpha1_resource_proto_msgTypes[3]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use LabelTerm.ProtoReflect.Descriptor instead.
func (*LabelTerm) Descriptor() ([]byte, []int) {
	return file_v1alpha1_resource_proto_rawDescGZIP(), []int{3}
}

func (x *LabelTerm) GetKey() string {
	if x != nil {
		return x.Key
	}
	return ""
}

func (x *LabelTerm) GetOp() LabelTerm_Operation {
	if x != nil {
		return x.Op
	}
	return LabelTerm_EXISTS
}

func (x *LabelTerm) GetValue() []string {
	if x != nil {
		return x.Value
	}
	return nil
}

func (x *LabelTerm) GetInvert() bool {
	if x != nil {
		return x.Invert
	}
	return false
}

// LabelQuery is a query on resource metadata labels.
//
// Terms are combined with AND.
type LabelQuery struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Terms         []*LabelTerm           `protobuf:"bytes,1,rep,name=terms,proto3" json:"terms,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *LabelQuery) Reset() {
	*x = LabelQuery{}
	mi := &file_v1alpha1_resource_proto_msgTypes[4]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *LabelQuery) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*LabelQuery) ProtoMessage() {}

func (x *LabelQuery) ProtoReflect() protoreflect.Message {
	mi := &file_v1alpha1_resource_proto_msgTypes[4]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use LabelQuery.ProtoReflect.Descriptor instead.
func (*LabelQuery) Descriptor() ([]byte, []int) {
	return file_v1alpha1_resource_proto_rawDescGZIP(), []int{4}
}

func (x *LabelQuery) GetTerms() []*LabelTerm {
	if x != nil {
		return x.Terms
	}
	return nil
}

// IDQuery is a query on resource metadata ID.
type IDQuery struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Regexp        string                 `protobuf:"bytes,1,opt,name=regexp,proto3" json:"regexp,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *IDQuery) Reset() {
	*x = IDQuery{}
	mi := &file_v1alpha1_resource_proto_msgTypes[5]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *IDQuery) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*IDQuery) ProtoMessage() {}

func (x *IDQuery) ProtoReflect() protoreflect.Message {
	mi := &file_v1alpha1_resource_proto_msgTypes[5]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use IDQuery.ProtoReflect.Descriptor instead.
func (*IDQuery) Descriptor() ([]byte, []int) {
	return file_v1alpha1_resource_proto_rawDescGZIP(), []int{5}
}

func (x *IDQuery) GetRegexp() string {
	if x != nil {
		return x.Regexp
	}
	return ""
}

var File_v1alpha1_resource_proto protoreflect.FileDescriptor

const file_v1alpha1_resource_proto_rawDesc = "" +
	"\n" +
	"\x17v1alpha1/resource.proto\x12\rcosi.resource\x1a\x1fgoogle/protobuf/timestamp.proto\"\xa2\x04\n" +
	"\bMetadata\x12\x1c\n" +
	"\tnamespace\x18\x01 \x01(\tR\tnamespace\x12\x12\n" +
	"\x04type\x18\x02 \x01(\tR\x04type\x12\x0e\n" +
	"\x02id\x18\x03 \x01(\tR\x02id\x12\x18\n" +
	"\aversion\x18\x04 \x01(\tR\aversion\x12\x14\n" +
	"\x05owner\x18\x05 \x01(\tR\x05owner\x12\x14\n" +
	"\x05phase\x18\x06 \x01(\tR\x05phase\x124\n" +
	"\acreated\x18\a \x01(\v2\x1a.google.protobuf.TimestampR\acreated\x124\n" +
	"\aupdated\x18\b \x01(\v2\x1a.google.protobuf.TimestampR\aupdated\x12\x1e\n" +
	"\n" +
	"finalizers\x18\t \x03(\tR\n" +
	"finalizers\x12J\n" +
	"\vannotations\x18\v \x03(\v2(.cosi.resource.Metadata.AnnotationsEntryR\vannotations\x12;\n" +
	"\x06labels\x18\n" +
	" \x03(\v2#.cosi.resource.Metadata.LabelsEntryR\x06labels\x1a>\n" +
	"\x10AnnotationsEntry\x12\x10\n" +
	"\x03key\x18\x01 \x01(\tR\x03key\x12\x14\n" +
	"\x05value\x18\x02 \x01(\tR\x05value:\x028\x01\x1a9\n" +
	"\vLabelsEntry\x12\x10\n" +
	"\x03key\x18\x01 \x01(\tR\x03key\x12\x14\n" +
	"\x05value\x18\x02 \x01(\tR\x05value:\x028\x01\"B\n" +
	"\x04Spec\x12\x1d\n" +
	"\n" +
	"proto_spec\x18\x01 \x01(\fR\tprotoSpec\x12\x1b\n" +
	"\tyaml_spec\x18\x02 \x01(\tR\byamlSpec\"h\n" +
	"\bResource\x123\n" +
	"\bmetadata\x18\x01 \x01(\v2\x17.cosi.resource.MetadataR\bmetadata\x12'\n" +
	"\x04spec\x18\x02 \x01(\v2\x13.cosi.resource.SpecR\x04spec\"\xf1\x01\n" +
	"\tLabelTerm\x12\x10\n" +
	"\x03key\x18\x01 \x01(\tR\x03key\x122\n" +
	"\x02op\x18\x02 \x01(\x0e2\".cosi.resource.LabelTerm.OperationR\x02op\x12\x14\n" +
	"\x05value\x18\x03 \x03(\tR\x05value\x12\x16\n" +
	"\x06invert\x18\x05 \x01(\bR\x06invert\"p\n" +
	"\tOperation\x12\n" +
	"\n" +
	"\x06EXISTS\x10\x00\x12\t\n" +
	"\x05EQUAL\x10\x01\x12\x12\n" +
	"\n" +
	"NOT_EXISTS\x10\x02\x1a\x02\b\x01\x12\x06\n" +
	"\x02IN\x10\x03\x12\x06\n" +
	"\x02LT\x10\x04\x12\a\n" +
	"\x03LTE\x10\x05\x12\x0e\n" +
	"\n" +
	"LT_NUMERIC\x10\x06\x12\x0f\n" +
	"\vLTE_NUMERIC\x10\a\"<\n" +
	"\n" +
	"LabelQuery\x12.\n" +
	"\x05terms\x18\x01 \x03(\v2\x18.cosi.resource.LabelTermR\x05terms\"!\n" +
	"\aIDQuery\x12\x16\n" +
	"\x06regexp\x18\x01 \x01(\tR\x06regexpB.Z,github.com/cosi-project/runtime/api/v1alpha1b\x06proto3"

var (
	file_v1alpha1_resource_proto_rawDescOnce sync.Once
	file_v1alpha1_resource_proto_rawDescData []byte
)

func file_v1alpha1_resource_proto_rawDescGZIP() []byte {
	file_v1alpha1_resource_proto_rawDescOnce.Do(func() {
		file_v1alpha1_resource_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_v1alpha1_resource_proto_rawDesc), len(file_v1alpha1_resource_proto_rawDesc)))
	})
	return file_v1alpha1_resource_proto_rawDescData
}

var file_v1alpha1_resource_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_v1alpha1_resource_proto_msgTypes = make([]protoimpl.MessageInfo, 8)
var file_v1alpha1_resource_proto_goTypes = []any{
	(LabelTerm_Operation)(0),      // 0: cosi.resource.LabelTerm.Operation
	(*Metadata)(nil),              // 1: cosi.resource.Metadata
	(*Spec)(nil),                  // 2: cosi.resource.Spec
	(*Resource)(nil),              // 3: cosi.resource.Resource
	(*LabelTerm)(nil),             // 4: cosi.resource.LabelTerm
	(*LabelQuery)(nil),            // 5: cosi.resource.LabelQuery
	(*IDQuery)(nil),               // 6: cosi.resource.IDQuery
	nil,                           // 7: cosi.resource.Metadata.AnnotationsEntry
	nil,                           // 8: cosi.resource.Metadata.LabelsEntry
	(*timestamppb.Timestamp)(nil), // 9: google.protobuf.Timestamp
}
var file_v1alpha1_resource_proto_depIdxs = []int32{
	9, // 0: cosi.resource.Metadata.created:type_name -> google.protobuf.Timestamp
	9, // 1: cosi.resource.Metadata.updated:type_name -> google.protobuf.Timestamp
	7, // 2: cosi.resource.Metadata.annotations:type_name -> cosi.resource.Metadata.AnnotationsEntry
	8, // 3: cosi.resource.Metadata.labels:type_name -> cosi.resource.Metadata.LabelsEntry
	1, // 4: cosi.resource.Resource.metadata:type_name -> cosi.resource.Metadata
	2, // 5: cosi.resource.Resource.spec:type_name -> cosi.resource.Spec
	0, // 6: cosi.resource.LabelTerm.op:type_name -> cosi.resource.LabelTerm.Operation
	4, // 7: cosi.resource.LabelQuery.terms:type_name -> cosi.resource.LabelTerm
	8, // [8:8] is the sub-list for method output_type
	8, // [8:8] is the sub-list for method input_type
	8, // [8:8] is the sub-list for extension type_name
	8, // [8:8] is the sub-list for extension extendee
	0, // [0:8] is the sub-list for field type_name
}

func init() { file_v1alpha1_resource_proto_init() }
func file_v1alpha1_resource_proto_init() {
	if File_v1alpha1_resource_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_v1alpha1_resource_proto_rawDesc), len(file_v1alpha1_resource_proto_rawDesc)),
			NumEnums:      1,
			NumMessages:   8,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_v1alpha1_resource_proto_goTypes,
		DependencyIndexes: file_v1alpha1_resource_proto_depIdxs,
		EnumInfos:         file_v1alpha1_resource_proto_enumTypes,
		MessageInfos:      file_v1alpha1_resource_proto_msgTypes,
	}.Build()
	File_v1alpha1_resource_proto = out.File
	file_v1alpha1_resource_proto_goTypes = nil
	file_v1alpha1_resource_proto_depIdxs = nil
}
