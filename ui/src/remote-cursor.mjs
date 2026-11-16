// noVNC hides the local cursor until the server supplies a visible shape.
// Headless macOS guests may never do so, so keep a native arrow as a fallback.
export function installLocalCursorFallback(cursor, enabled) {
  if (!enabled) return;

  const change = cursor.change.bind(cursor);
  const clear = cursor.clear.bind(cursor);
  const fallback = () => {
    clear();
    cursor._target.style.cursor = "default";
  };

  cursor.clear = fallback;
  cursor.change = (rgba, hotx, hoty, w, h) => {
    for (let i = 3; i < rgba.length; i += 4) {
      if (rgba[i] !== 0) {
        change(rgba, hotx, hoty, w, h);
        return;
      }
    }
    fallback();
  };
}

// 远端像素映射到 CSS 像素，指针和画面使用相同的缩放比例。
export function scaleRemoteCursor(cursor, surface) {
  const change = cursor.change.bind(cursor);
  let shape;
  const render = () => {
    if (!shape || !cursor._target) return;
    const { rgba, hotx, hoty, width, height } = shape;
    const canvas = surface();
    const bounds = canvas?.getBoundingClientRect();
    if (!bounds?.width || !canvas.width || !width || !height) {
      change(rgba, hotx, hoty, width, height);
      return;
    }
    const ratio = bounds.width / canvas.width;
    const targetWidth = Math.max(1, Math.round(width * ratio));
    const targetHeight = Math.max(1, Math.round(height * ratio));
    const source = document.createElement("canvas");
    source.width = width; source.height = height;
    source.getContext("2d").putImageData(new ImageData(new Uint8ClampedArray(rgba), width, height), 0, 0);
    const scaled = document.createElement("canvas");
    scaled.width = targetWidth; scaled.height = targetHeight;
    const context = scaled.getContext("2d");
    context.drawImage(source, 0, 0, targetWidth, targetHeight);
    change(context.getImageData(0, 0, targetWidth, targetHeight).data,
      Math.min(targetWidth - 1, Math.round(hotx * ratio)),
      Math.min(targetHeight - 1, Math.round(hoty * ratio)), targetWidth, targetHeight);
  };
  cursor.change = (rgba, hotx, hoty, width, height) => {
    shape = { rgba: new Uint8ClampedArray(rgba), hotx, hoty, width, height };
    render();
  };
  const clear = cursor.clear.bind(cursor);
  cursor.clear = () => { shape = undefined; clear(); };
  return render;
}
