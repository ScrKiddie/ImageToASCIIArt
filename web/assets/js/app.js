document.addEventListener('DOMContentLoaded', () => {
    const MAX_FILE_SIZE = 50 * 1024 * 1024;
    const SUPPORTED_FORMATS = ['image/jpeg', 'image/png'];
    let originalImageData = null, isProcessing = false, wasmLoaded = false, debounceTimer, lastSvgUrl = null;

    const $ = (id) => document.getElementById(id);
    const DOM = {
        imageInput: $('image'), fileLabel: $('file-label'), fileInfo: $('file-info'),
        progressBar: $('progress-bar'), progressFill: $('progress-fill'),
        controlsContainer: $('controls-container'), colorControlsContainer: $('color-controls-container'),
        resultContent: $('result-content'), loadingOverlay: $('loading-overlay'), themeToggle: $('theme-toggle'),
        bgColorPicker: $('background-color'), bgColorText: $('background-color-text'),
        bgPresetColors: document.querySelectorAll('#bg-preset-colors .preset-color'),
        transparencySection: $('transparency-section'), transparencyColorPicker: $('transparency-color'),
        transparencyColorText: $('transparency-color-text'),
        transparencyPresetColors: document.querySelectorAll('#transparency-preset-colors .preset-color'),
        downloadSvgBtn: $('download-svg-btn'), downloadPngBtn: $('download-png-btn'), downloadJpgBtn: $('download-jpg-btn'),
        sliders: {
            width: $('width'), brightness: $('brightness'), contrast: $('contrast'),
            sharpen: $('sharpen'), transparencyThreshold: $('transparency-threshold'),
        },
        valueDisplays: {
            width: $('width-value'), brightness: $('brightness-value'), contrast: $('contrast-value'),
            sharpen: $('sharpen-value'), transparencyThreshold: $('transparency-threshold-value'),
        }
    };
    DOM.themeToggle.checked = document.documentElement.classList.contains('dark-mode');
    const setDownloadButtonsState = (disabled) => [DOM.downloadSvgBtn, DOM.downloadPngBtn, DOM.downloadJpgBtn].forEach(btn => btn.disabled = disabled);
    setDownloadButtonsState(true);

    const formatFileSize = (bytes) => {
        if (bytes === 0) return '0 Bytes';
        const k = 1024, i = Math.floor(Math.log(bytes) / Math.log(k));
        return `${parseFloat((bytes / Math.pow(k, i)).toFixed(2))} ${['Bytes', 'KB', 'MB', 'GB'][i]}`;
    };

    const showMessage = (message, type = 'info', duration = 5000) => {
        const toast = document.createElement('div');
        toast.className = `toast toast-${type}`;
        toast.innerHTML = `<strong>${type.charAt(0).toUpperCase() + type.slice(1)}:</strong> ${message}`;
        $('toast-container').appendChild(toast);
        setTimeout(() => toast.classList.add('show'), 100);
        setTimeout(() => {
            toast.classList.remove('show');
            toast.addEventListener('transitionend', () => toast.remove());
        }, duration);
    };

    const processAndDisplay = async () => {
        if (!originalImageData || !wasmLoaded || isProcessing) return;
        isProcessing = true;
        DOM.loadingOverlay.classList.add('active');
        setDownloadButtonsState(true);

        setTimeout(async () => {
            try {
                const params = {
                    width: parseInt(DOM.sliders.width.value, 10),
                    brightness: parseFloat(DOM.sliders.brightness.value),
                    contrast: parseFloat(DOM.sliders.contrast.value),
                    sharpen: parseFloat(DOM.sliders.sharpen.value),
                    backgroundColor: DOM.bgColorPicker.value,
                    transparencyColor: DOM.transparencyColorPicker.value,
                    transparencyThreshold: parseFloat(DOM.sliders.transparencyThreshold.value),
                };

                const svgData = await window.processImageGo(originalImageData, ...Object.values(params));
                if (!svgData) throw new Error('Generated SVG data is empty.');

                if (lastSvgUrl) URL.revokeObjectURL(lastSvgUrl);
                lastSvgUrl = URL.createObjectURL(new Blob([svgData], { type: 'image/svg+xml' }));

                DOM.resultContent.innerHTML = `<img src="${lastSvgUrl}" alt="Generated ASCII SVG">`;
                setDownloadButtonsState(false);
            } catch (error) {
                console.error('Processing error:', error);
                let msg = `Processing failed: ${error.message}`;
                if (error.message.includes('terlalu besar')) msg = 'Output too large. Try reducing the ASCII width.';
                else if (error.message.includes('gagal decode')) msg = 'Failed to decode image. Please check the file.';
                showMessage(msg, 'error');
            } finally {
                DOM.loadingOverlay.classList.remove('active');
                isProcessing = false;
            }
        }, 50);
    };

    const debouncedProcess = () => (clearTimeout(debounceTimer), debounceTimer = setTimeout(processAndDisplay, 300));

    const downloadResult = (format) => {
        if (!lastSvgUrl) return showMessage('No result to download.', 'error');
        const link = document.createElement('a');
        link.download = `ascii-art.${format}`;
        if (format === 'svg') {
            link.href = lastSvgUrl;
            link.click();
            return;
        }
        const img = new Image();
        img.onload = () => {
            const canvas = document.createElement('canvas');
            canvas.width = img.naturalWidth;
            canvas.height = img.naturalHeight;
            const ctx = canvas.getContext('2d');
            if (format === 'jpg') {
                ctx.fillStyle = DOM.bgColorPicker.value || '#000000';
                ctx.fillRect(0, 0, canvas.width, canvas.height);
            }
            ctx.drawImage(img, 0, 0);
            link.href = canvas.toDataURL(format === 'png' ? 'image/png' : 'image/jpeg', 0.9);
            link.click();
        };
        img.onerror = () => showMessage('Failed to convert image for download.', 'error');
        img.src = lastSvgUrl;
    };

    const handleImageChange = async (event) => {
        const file = event.target.files[0];
        if (!file) return showMessage('Please select an image file.', 'error');
        if (!SUPPORTED_FORMATS.includes(file.type)) return showMessage(`Unsupported file type: ${file.type}.`, 'error');
        if (file.size > MAX_FILE_SIZE) return showMessage(`File size (${formatFileSize(file.size)}) exceeds limit.`, 'error');

        DOM.fileLabel.textContent = 'Loading...';
        DOM.fileLabel.disabled = true;
        setDownloadButtonsState(true);
        DOM.fileInfo.innerHTML = `<strong>File:</strong> ${file.name}<br><strong>Size:</strong> ${formatFileSize(file.size)}`;

        const isPng = file.type === 'image/png';
        DOM.transparencySection.classList.toggle('disabled', !isPng);
        if (isPng) showMessage('PNG file detected. Use Transparency options to customize.', 'info', 7000);

        try {
            const img = await new Promise((resolve, reject) => {
                const image = new Image();
                image.onload = () => resolve(image);
                image.onerror = () => reject(new Error('Failed to load image.'));
                image.src = URL.createObjectURL(file);
            });

            if (img.width > 4096 || img.height > 4096) showMessage(`Image is large and will be resized.`, 'warning');

            const reader = new FileReader();
            reader.onload = (e) => {
                originalImageData = new Uint8Array(e.target.result);
                DOM.fileLabel.textContent = 'Change Image';
                [DOM.controlsContainer, DOM.colorControlsContainer].forEach(c => c.classList.add('active'));

                const suggestedWidth = Math.min(Math.max(Math.round(100 * (img.width / img.height)), 50), 200);
                DOM.sliders.width.value = suggestedWidth;
                DOM.valueDisplays.width.textContent = suggestedWidth;

                processAndDisplay();
            };
            reader.onerror = () => showMessage('Failed to read file.', 'error');
            reader.readAsArrayBuffer(file);
        } catch (error) {
            showMessage(`Failed to load image: ${error.message}`, 'error');
        } finally {
            DOM.fileLabel.disabled = false;
        }
    };

    const setupColorPicker = (picker, text, presets) => {
        const update = (color) => {
            picker.value = color;
            text.value = color;
            presets.forEach(p => p.classList.toggle('active', p.dataset.color.toLowerCase() === color.toLowerCase()));
            debouncedProcess();
        };
        picker.addEventListener('input', () => (text.value = picker.value, update(picker.value)));
        text.addEventListener('blur', () => (/^#([A-Fa-f0-9]{6}|[A-Fa-f0-9]{3})$/.test(text.value) ? update(text.value) : (showMessage('Invalid hex color.', 'error'), text.value = picker.value)));
        presets.forEach(p => p.addEventListener('click', () => update(p.dataset.color)));
    };

    setupColorPicker(DOM.bgColorPicker, DOM.bgColorText, DOM.bgPresetColors);
    setupColorPicker(DOM.transparencyColorPicker, DOM.transparencyColorText, DOM.transparencyPresetColors);

    Object.entries(DOM.sliders).forEach(([key, slider]) => {
        slider.addEventListener('input', () => {
            const value = parseFloat(slider.value);
            DOM.valueDisplays[key].textContent = (key === 'sharpen' || key === 'transparencyThreshold') ? value.toFixed(2) : value;
        });
        slider.addEventListener('change', processAndDisplay);
    });

    DOM.imageInput.addEventListener('change', handleImageChange);
    DOM.themeToggle.addEventListener('change', () => {
        const isDark = DOM.themeToggle.checked;
        document.documentElement.classList.toggle('dark-mode', isDark);
        localStorage.setItem('theme', isDark ? 'dark' : 'light');
    });
    DOM.downloadSvgBtn.addEventListener('click', () => downloadResult('svg'));
    DOM.downloadPngBtn.addEventListener('click', () => downloadResult('png'));
    DOM.downloadJpgBtn.addEventListener('click', () => downloadResult('jpg'));

    (async function initWasm() {
        try {
            const go = new Go();
            const result = await WebAssembly.instantiateStreaming(fetch("main.wasm"), go.importObject);
            go.run(result.instance);
            wasmLoaded = true;
            showMessage('Converter is ready!', 'success', 3000);
        } catch (err) {
            console.error("Wasm initialization failed:", err);
            showMessage('Failed to load core component. Please refresh.', 'error', 10000);
            DOM.fileLabel.disabled = true;
            DOM.fileLabel.textContent = 'Wasm Loading Failed';
        }
    })();
});