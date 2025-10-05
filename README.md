
# ImageToASCIIArt

<div align="center">
<img
src="https://github.com/user-attachments/assets/a7900037-8298-4240-96f9-d2d8c4de4b22"
style="max-height: 500px; height: auto; width: auto;"
/>
</div>

ImageToASCIIArt is a client-side image to ASCII art converter built with Go and WebAssembly. Transform your images into text art directly in your browser, with complete privacy.

## Live Demo

**[https://scrkiddie.github.io/ImageToASCIIArt/](https://scrkiddie.github.io/ImageToASCIIArt/)**

## Features

* **Client-Side Processing**: All conversions happen in your browser. Your images are never uploaded to a server, ensuring 100% privacy.
* **Rich Customization**: Adjust ASCII width, brightness, contrast, and sharpening to get the perfect result.
* **Colorful Output**: Preserves the colors of your original image in the ASCII art.
* **Custom Backgrounds**: Choose any color for the background of your ASCII art.
* **Transparency Handling**: For PNG images, replace transparent areas with a custom color and control the transparency threshold.
* **Multiple Exports**: Download your creation as `SVG`, `PNG`, or `JPG`.
* **Responsive Design**: Works seamlessly on desktop and mobile devices.

## Local Setup

Follow these steps to run or modify the project locally.

### Prerequisites

- [Go](https://golang.org/doc/install) (version 1.24 or newer)
- [Git](https://git-scm.com/book/en/v2/Getting-Started-Installing-Git)

### Build Commands

1. Clone the repository and navigate into the project directory.

    ```bash
    git clone https://github.com/ScrKiddie/ImageToASCIIArt.git && cd ImageToASCIIArt
    ```

2. Compile the Go code into a WebAssembly module and place it directly into the `web` directory.

    ```bash
    GOOS=js GOARCH=wasm go build -o ./web/main.wasm ./main.go
    ```

3. Copy the necessary JavaScript runtime file to the `web` directory. This file is required to run Go WebAssembly modules.

    ```bash
    cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" ./web/
    ```

4. Serve the `web` directory using any local web server. For example, if you have Python installed, you can use its built-in server:

    ```bash
    cd web && python3 -m http.server 8000
    ```

    Once the server is running, open `http://localhost:8000` in your browser.

## License

This project is licensed under the **MIT License**.