package abi

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/eris-ltd/lllc-server/Godeps/_workspace/src/github.com/eris-ltd/go-ethereum/crypto"
)

var NullABI = ABI{}

// Callable method given a `Name` and whether the method is a constant.
// If the method is `Const` no transaction needs to be created for this
// particular Method call. It can easily be simulated using a local VM.
// For example a `Balance()` method only needs to retrieve something
// from the storage and therefor requires no Tx to be send to the
// network. A method such as `Transact` does require a Tx and thus will
// be flagged `true`.
// Input specifies the required input parameters for this gives method.
type Method struct {
	Name   string `json:"name"`
	Const  bool
	Input  []Argument `json:"inputs"`
	Return Type       // not yet implemented
}

// Returns the methods string signature according to the ABI spec.
//
// Example
//
//     function foo(uint32 a, int b)    =    "foo(uint32,int256)"
//
// Please note that "int" is substitute for its canonical representation "int256"
func (m Method) String() (out string) {
	if strings.Contains(m.Name, "(") && strings.Contains(m.Name, ")") {
		return m.Name
	}
	out += m.Name
	types := make([]string, len(m.Input))
	i := 0
	for _, input := range m.Input {
		types[i] = input.Type.String()
		i++
	}
	out += "(" + strings.Join(types, ",") + ")"

	return
}

func (m Method) Id() []byte {
	return crypto.Sha3([]byte(m.String()))[:4]
}

// Argument holds the name of the argument and the corresponding type.
// Types are used when packing and testing arguments.
type Argument struct {
	Name string
	Type Type
}

func (a *Argument) UnmarshalJSON(data []byte) error {
	var extarg struct {
		Name string
		Type string
	}
	err := json.Unmarshal(data, &extarg)
	if err != nil {
		return fmt.Errorf("argument json err: %v", err)
	}

	a.Type, err = NewType(extarg.Type)
	if err != nil {
		return err
	}
	a.Name = extarg.Name

	return nil
}

// The ABI holds information about a contract's context and available
// invokable methods. It will allow you to type check function calls and
// packs data accordingly.
type ABI struct {
	Methods map[string]Method
}

// tests, tests whether the given input would result in a successful
// call. Checks argument list count and matches input to `input`.
func (abi ABI) pack(name string, args ...interface{}) ([]byte, error) {
	method := abi.Methods[name]

	var ret []byte
	for i, a := range args {
		input := method.Input[i]

		packed, err := input.Type.pack(a)
		if err != nil {
			return nil, fmt.Errorf("`%s` %v", name, err)
		}
		ret = append(ret, packed...)

	}

	return ret, nil
}

// Pack the given method name to conform the ABI. Method call's data
// will consist of method_id, args0, arg1, ... argN. Method id consists
// of 4 bytes and arguments are all 32 bytes.
// Method ids are created from the first 4 bytes of the hash of the
// methods string signature. (signature = baz(uint32,string32))
func (abi ABI) Pack(name string, args ...interface{}) ([]byte, error) {
	method, exist := abi.Methods[name]
	if !exist {
		return nil, fmt.Errorf("method '%s' not found", name)
	}

	// start with argument count match
	if len(args) != len(method.Input) {
		return nil, fmt.Errorf("argument count mismatch: %d for %d", len(args), len(method.Input))
	}

	arguments, err := abi.pack(name, args...)
	if err != nil {
		return nil, err
	}

	// Set function id
	packed := abi.Methods[name].Id()
	packed = append(packed, arguments...)

	return packed, nil
}

func (abi *ABI) UnmarshalJSON(data []byte) error {
	var methods []Method
	if err := json.Unmarshal(data, &methods); err != nil {
		return err
	}

	abi.Methods = make(map[string]Method)
	for _, method := range methods {
		spl := strings.SplitN(method.Name, "(", 2)
		abi.Methods[spl[0]] = method
	}

	return nil
}

func JSON(reader io.Reader) (ABI, error) {
	dec := json.NewDecoder(reader)

	var abi ABI
	if err := dec.Decode(&abi); err != nil {
		return ABI{}, err
	}

	return abi, nil
}
