// import React from "react";
import * as ReactDOM from "react-dom";

function App() {
  return (
    <>
      <h1>foo</h1>
      <iframe
        src="https://localhost:12345/"
        style={{ position: "absolute", top: 0, left: 0, border: "none" }}
        width="100%"
        height="100%"
        allow="clipboard-read; clipboard-write"
      />
    </>
  );
}

const elem = document.createElement("div");
document.body.appendChild(elem);
ReactDOM.render(<App />, elem);
