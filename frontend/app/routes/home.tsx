import type { JSX } from "react";
import type { Route } from "./+types/home";
import Navbar from "~/components/Navbar";

export function meta({}: Route.MetaArgs) {
  return [
    { title: "synkademy" },
    { name: "description", content: "video streaming made eazy" },
  ];
}

export default function Home(): JSX.Element {
  return (
    <main>
      <section>
        <Navbar />
        <div>Synkademy!!!</div>
      </section>
    </main>
  );
}
