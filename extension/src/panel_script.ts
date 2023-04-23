const frame = Object.assign(document.createElement("iframe"), {
    src: "http://localhost:12345/",
    style: "position: absolute; top: 0; left: 0; border: none;",
    width: "100%",
    height: "100%",
    allow: "clipboard-read; clipboard-write",
});

document.body.appendChild(frame);