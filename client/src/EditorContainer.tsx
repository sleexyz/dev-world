import { useEffect } from "react";
import { useMatch } from "react-router-dom";

export function EditorContainer() {
  const alias = useMatch("/:alias")?.params.alias;

  return (
    <>
      <iframe
        src={`/workspace?alias=${alias}`}
        style={{ position: "absolute", top: 0, left: 0, border: "none" }}
        width="100%"
        height="100%"
        allow="clipboard-read; clipboard-write"
      />
    </>
  );
}