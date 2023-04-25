import { useEffect } from "react";
import { useMatch } from "react-router-dom";

export function EditorContainer() {
  const alias = useMatch("/:alias")?.params.alias;

  useEffect(() => {
    document.title = `dev/${alias}`;
  }, []);

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

// Do not refresh the page so we can edit dev-world from within dev-world :)
if (import.meta.hot) {
  import.meta.hot.accept(() => import.meta.hot.invalidate());
}
