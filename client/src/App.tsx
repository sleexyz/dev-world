import useSWR, { mutate } from "swr";
import { Menu, Transition } from "@headlessui/react";
import { Fragment } from "react";
import { Link } from "react-router-dom";

type DebugResponse = {
  workspaces: string[];
};

function App() {
  const response = useSWR(
    "/__api__/workspaces",
    async (url) => {
      const res = await fetch(url);
      const data = (await res.json()) as DebugResponse;
      // Sort the workspaces by path:
      return {
        ...data,
        workspaces: data.workspaces.sort(),
      };
    },
    {
      refreshInterval: 0,
    }
  );

  if (response.error) {
    return <div className="container"> {response.error.message} </div>;
  }
  if (!response.data) {
    return <div className="container"> loading... </div>;
  }
  return (
    <div className="container">
      <div className="space-y-8 mt-8">
        {response.data.workspaces.map((path, i) => (
          <Workspace key={i} path={path} />
        ))}
      </div>
    </div>
  );
}

function Workspace({ path }: { path: string }) {
  const alias = path.split("/").pop();
  const link = `/${alias}`;
  return (
    <div className="flex justify-between">
      <Link to={link}>
        <h2><span className="opacity-20">dev/</span>{alias}</h2>
      </Link>
      <ActiveLinkMenu path={path} />
    </div>
  );
}

async function deleteWorkspace(path: string): Promise<DebugResponse> {
  const query = new URLSearchParams({ folder: path }).toString();
  const resp = await fetch(`/__api__/workspace?${query}`, {
    method: "DELETE",
  });
  return resp.json();
}

function ActiveLinkMenu({ path }: { path: string }) {
  function handleDeleteWorkspace() {
    mutate("/__api__/workspaces", deleteWorkspace(path), {
      optimisticData: (data: DebugResponse) => ({
        ...data,
        workspaces: data.workspaces.filter((p) => p !== path),
      }),
      rollbackOnError: true,
    });
  }
  return (
    <Menu as="div" className="relative inline-block text-left">
      <Menu.Button className="opacity-20">...</Menu.Button>
      <Transition
        as={Fragment}
        enter="transition ease-out duration-100"
        enterFrom="transform opacity-0 scale-95"
        enterTo="transform opacity-100 scale-100"
        leave="transition ease-in duration-75"
        leaveFrom="transform opacity-100 scale-100"
        leaveTo="transform opacity-0 scale-95"
      >
        <Menu.Items className="absolute right-0 z-10 mt-2 w-56 origin-top-right rounded-md shadow-lg ring-1 bg-slate-600 ring-white ring-opacity-5 focus:outline-none">
          <div className="py-1">
            <Menu.Item>
              <button
                className="block px-4 py-2 text-sm"
                onClick={handleDeleteWorkspace}
              >
                Close session
              </button>
            </Menu.Item>
          </div>
        </Menu.Items>
      </Transition>
    </Menu>
  );
}

export default App;
