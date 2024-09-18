/*
 * Copyright 2024 Hypermode, Inc.
 */

package assemblyscript

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"hypruntime/langsupport"
	"hypruntime/utils"

	"github.com/spf13/cast"
)

func (p *planner) NewStringHandler(typ string, rt reflect.Type) (langsupport.TypeHandler, error) {
	handler := NewTypeHandler[stringHandler](p, typ)
	handler.info = langsupport.NewTypeHandlerInfo(typ, rt, 4, 1)
	handler.nullable = _typeInfo.IsNullable(typ)
	return handler, nil
}

type stringHandler struct {
	info     langsupport.TypeHandlerInfo
	nullable bool
}

func (h *stringHandler) Info() langsupport.TypeHandlerInfo {
	return h.info
}

func (h *stringHandler) Read(ctx context.Context, wa langsupport.WasmAdapter, offset uint32) (any, error) {
	if offset == 0 {
		return nil, fmt.Errorf("unexpected address 0 reading managed object of type %s", h.info.TypeName())
	}

	ptr, ok := wa.Memory().ReadUint32Le(offset)
	if !ok {
		return nil, errors.New("failed to read string pointer")
	}

	return h.doReadString(wa, ptr)
}

func (h *stringHandler) Write(ctx context.Context, wa langsupport.WasmAdapter, offset uint32, obj any) (utils.Cleaner, error) {
	ptr, cln, err := h.doWriteString(ctx, wa, obj)
	if err != nil {
		return cln, err
	}

	if ok := wa.Memory().WriteUint32Le(offset, ptr); !ok {
		return cln, fmt.Errorf("failed to write string pointer to WASM memory")
	}

	return cln, nil
}

func (h *stringHandler) Decode(ctx context.Context, wa langsupport.WasmAdapter, vals []uint64) (any, error) {
	if len(vals) != 1 {
		return nil, fmt.Errorf("expected 1 value when decoding a string, got %d", len(vals))
	}

	return h.doReadString(wa, uint32(vals[0]))
}

func (h *stringHandler) Encode(ctx context.Context, wa langsupport.WasmAdapter, obj any) ([]uint64, utils.Cleaner, error) {
	ptr, cln, err := h.doWriteString(ctx, wa, obj)
	if err != nil {
		return nil, cln, err
	}

	return []uint64{uint64(ptr)}, cln, nil
}

func (h *stringHandler) doReadString(wa langsupport.WasmAdapter, offset uint32) (any, error) {
	if offset == 0 {
		if h.nullable {
			return nil, nil
		} else {
			return nil, errors.New("unexpected null pointer for non-nullable string")
		}
	}

	if id, ok := wa.Memory().ReadUint32Le(offset - 8); !ok {
		return nil, errors.New("failed to read class id of the WASM object")
	} else if id != 2 {
		return nil, errors.New("pointer is not to a string")
	}

	size, ok := wa.Memory().ReadUint32Le(offset - 4)
	if !ok {
		return nil, errors.New("failed to read string length")
	} else if size == 0 {
		return "", nil
	}

	bytes, ok := wa.Memory().Read(offset, size)
	if !ok {
		return nil, fmt.Errorf("failed to read string data from WASM memory (size: %d)", size)
	}

	str := utils.DecodeUTF16(bytes)
	return str, nil
}

func (h *stringHandler) doWriteString(ctx context.Context, wa langsupport.WasmAdapter, obj any) (uint32, utils.Cleaner, error) {
	if utils.HasNil(obj) {
		if h.nullable {
			return 0, nil, nil
		} else {
			return 0, nil, errors.New("unexpected nil value for non-nullable string")
		}
	}

	str, err := cast.ToStringE(obj)
	if err != nil {
		return 0, nil, err
	}

	bytes := utils.EncodeUTF16(str)
	return h.doWriteBytes(ctx, wa, bytes)
}

func (h *stringHandler) doWriteBytes(ctx context.Context, wa langsupport.WasmAdapter, bytes []byte) (uint32, utils.Cleaner, error) {
	const id = 2 // ID for string is always 2
	size := uint32(len(bytes))
	ptr, cln, err := wa.(*wasmAdapter).allocateAndPinMemory(ctx, size, id)
	if err != nil {
		return 0, cln, err
	}

	if ok := wa.Memory().Write(ptr, bytes); !ok {
		return 0, cln, fmt.Errorf("failed to write string data to WASM memory (size: %d)", size)
	}

	return ptr, cln, nil
}