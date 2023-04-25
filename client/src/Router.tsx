import App from "./App";
import { EditorContainer } from "./EditorContainer";
import { createBrowserRouter, RouterProvider } from "react-router-dom";

const router = createBrowserRouter([
  {
    path: "/",
    element: <App />,
  },
  {
    path: "/:alias",
    element : <EditorContainer/>,
  },
]);

export function Router() {
  return <RouterProvider router={router} />;
}
