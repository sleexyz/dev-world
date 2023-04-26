import App from "./App";
import { EditorContainer } from "./EditorContainer";
import {
  createBrowserRouter,
  Outlet,
  RouterProvider,
  useLocation,
} from "react-router-dom";
import { useEffect } from "react";

const router = createBrowserRouter([
  {
    path: "/",
    element: <Container />,
    children: [
      {
        index: true,
        element: <App />,
      },
      {
        path: ":alias",
        element: <EditorContainer />,
      },
    ],
  },
]);

function Container() {
  const location = useLocation();
  useEffect(() => {
    document.title = `${window.location.host}${window.location.pathname}`;
  }, [location]);
  return <Outlet/>;
}

export function Router() {
  return <RouterProvider router={router} />;
}
