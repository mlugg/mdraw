const DrawMode = {
	NONE:  0,
	DRAW:  1,
	ERASE: 2,
};

const DataType = {
	U8: 0,
	U16: 1,
	F32: 2,
};

function makeDrawContext(canvas, cursorCanvas, enableFunc, disableFunc) {
	let sock = new WebSocket("wss://" + location.host + "/draw/ws/foo");
	sock.binaryType = "arraybuffer";

	let ctx = {
		drawCtx:   canvas.getContext("2d"),
		cursorCtx: cursorCanvas.getContext("2d"),
		x: 0,
		y: 0,
		curMode:   DrawMode.NONE,
		forceMode: DrawMode.NONE,
		drawSize:  document.getElementById("drawSize").value,
		eraseSize: document.getElementById("eraseSize").value,
		//history: [],
		width:  canvas.width,
		height: canvas.height,
		socket: sock,
	};

	sock.addEventListener("message", e => sockMsg(ctx, e.data));

	sock.addEventListener("close", e => {
		disableFunc(ctx)
	});

	sock.addEventListener("open", e => {
		enableFunc(ctx);
		sync(ctx)
	});

	return ctx;
}

function sockMsg(ctx, msg) {
	const view = new DataView(msg);
	const cmd = view.getUint8(0);

	if (cmd == 0x00) { // CLEAR
		//ctx.history = [];
		clearCanvas(ctx);
	}

	if (cmd == 0x02) { // SYNC_DATA
		const b64 = btoa(String.fromCharCode(...new Uint8Array(msg.slice(1))));
		let img = new Image();
		img.src = "data:image/png;base64," + b64;
		img.onload = () => {
			ctx.drawCtx.drawImage(img, 0, 0);
		};
	}

	if (cmd == 0x03) { // DRAW
		const x0 = view.getUint16(1);
		const y0 = view.getUint16(3);
		const x1 = view.getUint16(5);
		const y1 = view.getUint16(7);
		const width = view.getFloat32(9);
		const r = view.getUint8(13);
		const g = view.getUint8(14);
		const b = view.getUint8(15);
		const a = view.getUint8(16) / 255;
		drawPacket(ctx, `rgba(${r},${g},${b},${a})`, width, x0, y0, x1, y1);
	}

	if (cmd == 0x04) { // ERASE
		const x0 = view.getUint16(1);
		const y0 = view.getUint16(3);
		const x1 = view.getUint16(5);
		const y1 = view.getUint16(7);
		const width = view.getFloat32(9);
		erasePacket(ctx, width, x0, y0, x1, y1);
	}
}

function drawPacket(ctx, c, w, x0, y0, x1, y1) {
	ctx.drawCtx.save();

	setColor(ctx, c);
	ctx.drawCtx.globalCompositeOperation = "source-over";
	canvasLine(ctx.drawCtx, w, x0, y0, x1, y1);

	ctx.drawCtx.restore();
}

function erasePacket(ctx, w, x0, y0, x1, y1) {
	ctx.drawCtx.save();

	ctx.drawCtx.globalCompositeOperation = "destination-out";
	canvasLine(ctx.drawCtx, w, x0, y0, x1, y1);

	ctx.drawCtx.restore();
}

function sendData(ctx, vals) {
	let len = 0;

	for (let i = 0; i < vals.length; i += 2) {
		switch (vals[i]) {
			case DataType.U8:
				len += 1;
				break;
			case DataType.U16:
				len += 2;
				break;
			case DataType.F32:
				len += 4;
				break;
		}
	}

	let buf = new ArrayBuffer(len);
	let view = new DataView(buf);
	let bufIdx = 0;

	for (let i = 0; i < vals.length; i += 2) {
		const type = vals[i];
		const val = vals[i+1];

		switch (type) {
			case DataType.U8:
				view.setUint8(bufIdx, val);
				bufIdx += 1;
				break;
			case DataType.U16:
				view.setUint16(bufIdx, val, false);
				bufIdx += 2;
				break;
			case DataType.F32:
				view.setFloat32(bufIdx, val, false);
				bufIdx += 4;
				break;
		}
	}

	ctx.socket.send(buf);
}

function sync(ctx) {
	sendData(ctx, [DataType.U8, 0x01]);
}

// Drawing functions {{{

// Internal {{{

function canvasLine(c, w, x0, y0, x1, y1) {
	c.beginPath();
	c.lineWidth = w;
	c.moveTo(x0, y0);
	c.lineTo(x1, y1);
	c.stroke();
	c.arc(x0, y0, w/2, 0, Math.PI*2);
	c.arc(x1, y1, w/2, 0, Math.PI*2);
	c.fill();
	c.closePath();
}

function parseColor(inp) {
	var m;

	if (m = inp.match(/^#([0-9a-f]{3})$/i)) {
		return [
			parseInt(m[1].charAt(0), 16) * 0x11,
			parseInt(m[1].charAt(1), 16) * 0x11,
			parseInt(m[1].charAt(2), 16) * 0x11,
		];
	}

	if (m = inp.match(/^#([0-9a-f]{6})$/i)) {
		return [
			parseInt(m[1].substr(0, 2), 16),
			parseInt(m[1].substr(2, 2), 16),
			parseInt(m[1].substr(4, 2), 16),
		];
	}

	return [0, 0, 0];
}

function drawLine(ctx, pressure, x0, y0, x1, y1) {
	const width = ctx.drawSize * pressure;
	const color = parseColor(ctx.drawCtx.fillStyle);
	sendData(ctx, [
		DataType.U8,  0x03,
		DataType.U16, x0,
		DataType.U16, y0,
		DataType.U16, x1,
		DataType.U16, y1,
		DataType.F32, width,
		DataType.U8,  color[0],
		DataType.U8,  color[1],
		DataType.U8,  color[2],
		DataType.U8,  255,
	]);
	ctx.drawCtx.globalCompositeOperation = "source-over";
	canvasLine(ctx.drawCtx, width, x0, y0, x1, y1);
}

function clearLine(ctx, pressure, x0, y0, x1, y1) {
	const width = ctx.eraseSize * pressure;
	sendData(ctx, [
		DataType.U8,  0x04,
		DataType.U16, x0,
		DataType.U16, y0,
		DataType.U16, x1,
		DataType.U16, y1,
		DataType.F32, width,
	]);
	ctx.drawCtx.globalCompositeOperation = "destination-out";
	canvasLine(ctx.drawCtx, width, x0, y0, x1, y1);
}

// }}}

function clear(ctx) {
	sendData(ctx, [DataType.U8, 0x00]);
	//ctx.history = [];
	clearCanvas(ctx);
}

function clearCanvas(ctx) {
	ctx.drawCtx.save();
	ctx.drawCtx.fillStyle = '#FFFFFF';
	ctx.drawCtx.fillRect(0,0,ctx.width,ctx.height);
	ctx.drawCtx.restore();
}

function setColor(ctx, c) {
	ctx.drawCtx.strokeStyle = c;
	ctx.drawCtx.fillStyle = c;
}

function undo(ctx) {
	/*
	if (ctx.history.length != 0) {
		ctx.socket.send("POP_HISTORY");
		const img = ctx.history.pop();
		ctx.drawCtx.putImageData(img, 0, 0);
	}
	*/
	alert("Not yet supported!");
}

function startDraw(ctx, pressure, x, y, mode) {
	//ctx.socket.send("PUSH_HISTORY");
	//ctx.history.push(ctx.drawCtx.getImageData(0, 0, ctx.width, ctx.height));

	if (ctx.forceMode == DrawMode.NONE) {
		ctx.curMode = mode;
	} else {
		ctx.curMode = ctx.forceMode;
	}

	ctx.x = x;
	ctx.y = y;

	doMove(ctx, pressure, ctx.x, ctx.y)
}

function endDraw(ctx) {
	ctx.curMode = DrawMode.NONE;
}

function doMove(ctx, pressure, newX, newY) {
	if (ctx.curMode == DrawMode.DRAW) {
		drawLine(ctx, pressure, ctx.x, ctx.y, newX, newY, true);
	} else if (ctx, ctx.curMode == DrawMode.ERASE) {
		clearLine(ctx, pressure, ctx.x, ctx.y, newX, newY, true);
	}
	ctx.x = newX;
	ctx.y = newY;
}

function doToolOverlay(ctx, pressure) {
	const c = ctx.cursorCtx;
	c.clearRect(0, 0, ctx.width, ctx.height);

	if (pressure == 0) pressure = 1;

	const mode = ctx.curMode   != DrawMode.NONE ? ctx.curMode
		: ctx.forceMode != DrawMode.NONE ? ctx.forceMode
		: DrawMode.DRAW;

	if (mode == DrawMode.ERASE) {
		const w = ctx.eraseSize * pressure;
		c.strokeStyle = "black";
		c.lineWidth = 1;
		c.beginPath();
		c.arc(ctx.x, ctx.y, w/2, 0, Math.PI*2);
		c.stroke();
		c.closePath();
	}

	if (mode == DrawMode.DRAW) {
		const w = ctx.drawSize * pressure;
		c.fillStyle = ctx.drawCtx.fillStyle;
		c.beginPath();
		c.arc(ctx.x, ctx.y, w/2, 0, Math.PI*2);
		c.fill();
		c.closePath();
	}
}

function clearToolOverlay(ctx) {
	ctx.cursorCtx.clearRect(0, 0, ctx.width, ctx.height);
}

// }}}
