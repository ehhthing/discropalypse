package acropalypse

import (
	"context"
	"fmt"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"io"
	"net/http"
)

const WasmUrl = "https://cdn.jsdelivr.net/gh/simontime/acropalypse-tool/acropalypse.wasm"

type Acropalypse struct {
	runtime     wazero.Runtime
	module      api.Module
	inputPtr    uint32
	outputPtr   uint32
	imageLength uint32
	Width       uint32
	Height      uint32
}

func malloc(ctx context.Context, mod api.Module, size uint32) uint32 {
	res, err := mod.ExportedFunction("f").Call(ctx, api.EncodeU32(size))
	if err != nil {
		panic(fmt.Errorf("failed to malloc %d bytes for image: %w", size, err))
	}
	return api.DecodeU32(res[0])
}

func acropalypseRecover(ctx context.Context, mod api.Module, inputPtr uint32, inputLen uint32, outputPtr uint32, width uint32, height uint32) int32 {
	res, err := mod.ExportedFunction("e").Call(
		ctx,
		api.EncodeU32(inputPtr),
		api.EncodeU32(inputLen),
		api.EncodeU32(outputPtr),
		api.EncodeU32(width),
		api.EncodeU32(height),
	)
	if err != nil {
		panic(fmt.Errorf("failed to run acropalypse: %w", err))
	}
	return api.DecodeI32(res[0])
}

func Fetch() ([]byte, error) {
	res, err := http.Get(WasmUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch acropalypse wasm: %w", err)
	}

	wasm, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch acropalypse wasm: %w", err)
	}

	return wasm, nil
}

func Init(ctx context.Context, wasm []byte, width uint32, height uint32) (*Acropalypse, error) {
	runtime := wazero.NewRuntime(ctx)
	compiled, err := runtime.CompileModule(ctx, wasm)
	if err != nil {
		return nil, fmt.Errorf("failed to compile acropalypse wasm: %w", err)
	}

	_, err = runtime.NewHostModuleBuilder("a").
		NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, size uint32) uint32 {
			delta := ((size - m.Memory().Size()) >> 16) + 1
			_, ok := m.Memory().Grow(delta)
			if !ok {
				panic(fmt.Errorf("failed to allocate memory for wasm heap"))
			}
			return 1
		}).Export("a").
		NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, dest uint32, src uint32, num uint32) {
			mem, ok := m.Memory().Read(src, num)
			if !ok {
				panic(fmt.Errorf("failed to copy memory from %d to %d (%d bytes)", src, dest, num))
			}
			m.Memory().Write(dest, mem)
		}).Export("b").
		Instantiate(context.Background())

	if err != nil {
		return nil, fmt.Errorf("failed to instantiate emscripten host module: %w", err)
	}

	mod, err := runtime.InstantiateModule(
		context.Background(),
		compiled,
		wazero.NewModuleConfig().WithName("acropalypse"),
	)
	if err != nil {
		panic(fmt.Errorf("failed to instantiate module: %w", err))
	}

	imageLength := ((width * 3) + 1) * height

	return &Acropalypse{
		inputPtr:    malloc(ctx, mod, imageLength),
		outputPtr:   malloc(ctx, mod, imageLength),
		runtime:     runtime,
		module:      mod,
		imageLength: imageLength,
		Width:       width,
		Height:      height,
	}, nil
}

func (m *Acropalypse) Recover(ctx context.Context, image []byte) ([]byte, error) {
	ok := m.module.Memory().Write(m.inputPtr, image)
	if !ok {
		return nil, fmt.Errorf("failed to write wasm memory")
	}

	result := acropalypseRecover(ctx, m.module, m.inputPtr, uint32(len(image)), m.outputPtr, m.Width, m.Height)
	if result < 0 {
		return nil, nil
	}

	memory, ok := m.module.Memory().Read(m.outputPtr, m.imageLength)
	if !ok {
		return nil, fmt.Errorf("failed to read wasm memory")
	}

	res := make([]byte, len(memory))
	copy(res, memory)

	return res, nil
}

func (m *Acropalypse) Close(ctx context.Context) error {
	return m.runtime.Close(ctx)
}
