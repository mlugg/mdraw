const DrawMode = {
	NONE:  0,
	DRAW:  1,
	ERASE: 2,
};

function makeDrawContext(canvas, cursorCanvas) {
	let sock = new WebSocket("wss://" + location.host + "/draw/ws/foo");

	let ctx = {
		drawCtx:   canvas.getContext("2d"),
		cursorCtx: cursorCanvas.getContext("2d"),
		x: 0,
		y: 0,
		curMode:   DrawMode.NONE,
		forceMode: DrawMode.NONE,
		drawSize:  document.getElementById("drawSize").value,
		eraseSize: document.getElementById("eraseSize").value,
		history: [],
		width:  canvas.width,
		height: canvas.height,
		socket: sock,
	};

	sock.addEventListener("message", e => sockMsg(ctx, e.data));

	return ctx;
}

function sockMsg(ctx, msg) {
	const parts = msg.split(" ");
	switch (parts[0]) {
		case "PUSH_HISTORY":
			ctx.history.push(ctx.drawCtx.getImageData(0, 0, ctx.width, ctx.height));
			break;
		case "POP_HISTORY":
			ctx.drawCtx.putImageData(ctx.history.pop(), 0, 0);
			break;
		case "CLEAR":
			ctx.history = [];
			ctx.drawCtx.clearRect(0, 0, ctx.width, ctx.height);
			break;
		case "DRAW":
			drawPacket(ctx, parts[1], parts[2], parts[3], parts[4], parts[5], parts[6]);
			break;
		case "ERASE":
			erasePacket(ctx, parts[1], parts[2], parts[3], parts[4], parts[5]);
			break;
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

function drawLine(ctx, pressure, x0, y0, x1, y1) {
	const w = ctx.drawSize * pressure;
	const c = ctx.drawCtx.fillStyle;
	ctx.socket.send(`DRAW ${c} ${w} ${x0} ${y0} ${x1} ${y1}`);
	ctx.drawCtx.globalCompositeOperation = "source-over";
	canvasLine(ctx.drawCtx, w, x0, y0, x1, y1);
}

function clearLine(ctx, pressure, x0, y0, x1, y1) {
	const w = ctx.eraseSize * pressure;
	ctx.socket.send(`ERASE ${w} ${x0} ${y0} ${x1} ${y1}`);
	ctx.drawCtx.globalCompositeOperation = "destination-out";
	canvasLine(ctx.drawCtx, w, x0, y0, x1, y1);
}

// }}}

function clear(ctx) {
	ctx.socket.send("CLEAR");
	ctx.history = [];
	ctx.drawCtx.clearRect(0, 0, ctx.width, ctx.height);
}

function setColor(ctx, c) {
	ctx.drawCtx.strokeStyle = c;
	ctx.drawCtx.fillStyle = c;
}

function undo(ctx) {
	if (ctx.history.length != 0) {
		ctx.socket.send("POP_HISTORY");
		const img = ctx.history.pop();
		ctx.drawCtx.putImageData(img, 0, 0);
	}
}

function startDraw(ctx, pressure, x, y, mode) {
	ctx.socket.send("PUSH_HISTORY");
	ctx.history.push(ctx.drawCtx.getImageData(0, 0, ctx.width, ctx.height));

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
