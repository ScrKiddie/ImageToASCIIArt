package main

import (
	"fmt"
	"image-to-ascii-art/lib"
	"syscall/js"
)

func validateImageParams(args []js.Value) ([]byte, lib.Options, error) {
	if len(args) != 8 {
		return nil, lib.Options{}, fmt.Errorf("expected 8 arguments, but got %d", len(args))
	}

	imageDataJS := args[0]
	if imageDataJS.IsNull() || imageDataJS.IsUndefined() {
		return nil, lib.Options{}, fmt.Errorf("imageData is null or undefined")
	}

	imageDataLength := imageDataJS.Get("length")
	if imageDataLength.IsNull() || imageDataLength.IsUndefined() {
		return nil, lib.Options{}, fmt.Errorf("imageData has no length property")
	}

	length := imageDataLength.Int()
	if length <= 0 {
		return nil, lib.Options{}, fmt.Errorf("imageData length is invalid: %d", length)
	}

	imageDataGo := make([]byte, length)
	js.CopyBytesToGo(imageDataGo, imageDataJS)

	opts := lib.Options{
		TargetWidth:           args[1].Int(),
		Brightness:            args[2].Float(),
		Contrast:              args[3].Float(),
		Sharpen:               args[4].Float(),
		BackgroundColor:       args[5].String(),
		TransparencyColor:     args[6].String(),
		TransparencyThreshold: args[7].Float(),
	}

	return imageDataGo, opts, nil
}

func processImage(imageDataGo []byte, opts lib.Options) (string, error) {
	js.Global().Get("console").Call("log",
		fmt.Sprintf("Processing image: width=%d, brightness=%.2f, contrast=%.2f, sharpen=%.2f, bg_color=%s, transparency_color=%s, threshold=%.2f",
			opts.TargetWidth, opts.Brightness, opts.Contrast, opts.Sharpen, opts.BackgroundColor, opts.TransparencyColor, opts.TransparencyThreshold))

	svgString, err := lib.ProcessImageToSVG(imageDataGo, opts)
	if err != nil {
		return "", fmt.Errorf("error processing image: %w", err)
	}

	js.Global().Get("console").Call("log", "Image processed successfully")
	return svgString, nil
}

func rejectWithError(reject js.Value, err error) {
	errorConstructor := js.Global().Get("Error")
	errorMsg := fmt.Sprintf("Error: %v", err)
	js.Global().Get("console").Call("error", errorMsg)
	errorObject := errorConstructor.New(errorMsg)
	reject.Invoke(errorObject)
}

func wrapperFunc() js.Func {
	return js.FuncOf(func(this js.Value, args []js.Value) any {
		handler := js.FuncOf(func(this js.Value, pArgs []js.Value) any {
			resolve := pArgs[0]
			reject := pArgs[1]

			go func() {
				defer func() {
					if r := recover(); r != nil {
						errorMsg := fmt.Sprintf("Panic in Go WASM: %v", r)
						rejectWithError(reject, fmt.Errorf(errorMsg))
					}
				}()

				imageDataGo, opts, err := validateImageParams(args)
				if err != nil {
					rejectWithError(reject, err)
					return
				}

				svgString, err := processImage(imageDataGo, opts)
				if err != nil {
					rejectWithError(reject, err)
					return
				}

				resolve.Invoke(svgString)
			}()

			return nil
		})

		promiseConstructor := js.Global().Get("Promise")
		return promiseConstructor.New(handler)
	})
}

func main() {
	js.Global().Get("console").Call("log", "Go WebAssembly Module Loaded")

	js.Global().Set("processImageGo", wrapperFunc())

	js.Global().Set("processImageGoSync", js.FuncOf(func(this js.Value, args []js.Value) any {
		imageDataGo, opts, err := validateImageParams(args)
		if err != nil {
			js.Global().Get("console").Call("error", fmt.Sprintf("Validation Error: %v", err))
			return ""
		}

		svgString, err := processImage(imageDataGo, opts)
		if err != nil {
			js.Global().Get("console").Call("error", fmt.Sprintf("Processing Error: %v", err))
			return ""
		}

		return svgString
	}))

	<-make(chan struct{})
}
