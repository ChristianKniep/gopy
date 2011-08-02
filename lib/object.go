// Copyright 2011 Julian Phillips.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package py

// #include <Python.h>
// #include "utils.h"
// static inline void incref(PyObject *obj) { Py_INCREF(obj); }
// static inline void decref(PyObject *obj) { Py_DECREF(obj); }
// static inline void xdecref(PyObject *obj) { Py_XDECREF(obj); }
// static inline uintptr_t headSize(void) { return sizeof(PyObject); }
import "C"

import (
	"fmt"
	"os"
	"unsafe"
)

type Object interface {
	Base() *BaseObject
	Type() *Type
	Decref()
	Incref()
	IsTrue() bool
	Not() bool
}

var None *NoneObject = &NoneObject{BaseObject{&C._Py_NoneStruct}}

type BaseObject struct {
	o *C.PyObject
}

type NoneObject struct {
	BaseObject
}

func newBaseObject(obj *C.PyObject) *BaseObject {
	if obj == nil {
		return nil
	}
	return &BaseObject{obj}
}

func c(obj Object) *C.PyObject {
	if obj == nil {
		return nil
	}
	return obj.Base().o
}

func (obj *BaseObject) Base() *BaseObject {
	return obj
}

func (obj *BaseObject) Type() *Type {
	o := obj.o.ob_type
	return newType((*C.PyObject)(unsafe.Pointer(o)))
}

func (obj *BaseObject) Decref() {
	C.decref(obj.o)
}

func (obj *BaseObject) Incref() {
	C.incref(obj.o)
}

func (obj *BaseObject) Call(args *Tuple, kwds *Dict) (Object, os.Error) {
	ret := C.PyObject_Call(c(obj), c(args), c(kwds))
	return obj2ObjErr(ret)
}

func (obj *BaseObject) CallObject(args *Tuple) (Object, os.Error) {
	var a *C.PyObject = nil
	if args != nil {
		a = c(args)
	}
	ret := C.PyObject_CallObject(c(obj), a)
	return obj2ObjErr(ret)
}

func (obj *BaseObject) CallFunction(format string, args ...interface{}) (Object, os.Error) {
	t, err := buildTuple(format, args...)
	if err != nil {
		return nil, err
	}
	return obj.CallObject(t)
}

func (obj *BaseObject) CallMethod(name string, format string, args ...interface{}) (Object, os.Error) {
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))

	f := C.PyObject_GetAttrString(obj.o, cname)
	if f == nil {
		return nil, fmt.Errorf("AttributeError: %s", name)
	}
	defer C.decref(f)

	if C.PyCallable_Check(f) == 0 {
		return nil, fmt.Errorf("TypeError: attribute of type '%s' is not callable", name)
	}

	t, err := buildTuple(format, args...)
	if err != nil {
		return nil, err
	}

	ret := C.PyObject_CallObject(f, c(t))
	return obj2ObjErr(ret)
}

func (obj *BaseObject) CallFunctionObjArgs(args ...Object) (Object, os.Error) {
	t, err := Tuple_Pack(args...)
	if err != nil {
		return nil, err
	}
	return obj.CallObject(t)
}

func (obj *BaseObject) CallMethodObjArgs(name string, args ...Object) (Object, os.Error) {
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))

	f := C.PyObject_GetAttrString(obj.o, cname)
	if f == nil {
		return nil, fmt.Errorf("AttributeError: %s", name)
	}
	defer C.decref(f)

	if C.PyCallable_Check(f) == 0 {
		return nil, fmt.Errorf("TypeError: attribute of type '%s' is not callable", name)
	}

	t, err := Tuple_Pack(args...)
	if err != nil {
		return nil, err
	}

	ret := C.PyObject_CallObject(f, c(t))
	return obj2ObjErr(ret)
}

func (obj *BaseObject) IsTrue() bool {
	ret := C.PyObject_IsTrue(c(obj))
	if ret < 0 {
		panic(exception())
	}
	return ret != 0
}

func (obj *BaseObject) Not() bool {
	ret := C.PyObject_Not(c(obj))
	if ret < 0 {
		panic(exception())
	}
	return ret != 0
}

func (obj *BaseObject) Dir() (Object, os.Error) {
	ret := C.PyObject_Dir(c(obj))
	return obj2ObjErr(ret)
}

var types = make(map[*C.PyTypeObject]*Class)

func registerType(pyType *C.PyTypeObject, class *Class) {
	types[pyType] = class
}

func (obj *BaseObject) actual() Object {
	class, ok := types[(*C.PyTypeObject)(c(obj).ob_type)]
	if ok {
		o := unsafe.Pointer(uintptr(unsafe.Pointer(c(obj))) + uintptr(C.headSize()))
		t := unsafe.Typeof(class.Pointer)
		ret, ok := unsafe.Unreflect(t, unsafe.Pointer(&o)).(Object)
		if ok {
			return ret
		}
	}
	switch C.getBasePyType(c(obj)) {
	case &C.PyList_Type:
		return &List{*obj}
	case &C.PyTuple_Type:
		return &Tuple{*obj}
	case &C.PyDict_Type:
		return &Dict{*obj}
	case &C.PyString_Type:
		return &String{*obj}
	case &C.PyBool_Type:
		return newBool(c(obj))
	case &C.PyInt_Type:
		return &Int{*obj}
	case &C.PyLong_Type:
		return &Long{*obj}
	case &C.PyFloat_Type:
		return &Float{*obj}
	case &C.PyModule_Type:
		return &Module{*obj}
	case &C.PyType_Type:
		return &Type{*obj}
	}
	return obj
}