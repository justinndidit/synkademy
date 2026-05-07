import type { JSX } from "react";
import { Link } from "react-router";

type NavbarProps = {
  canJoinSession?: boolean;
  canCreateSession?: boolean;
};

const Navbar = ({
  canJoinSession = true,
  canCreateSession = true,
}: NavbarProps): JSX.Element => {
  //const showJoinSessionBtn = canJoinSession ? "visible" : "hidden";
  return (
    <nav className="navbar flex justify-between">
      <Link className="flex " to="/">
        <img className="m-1.5" src="/video_icon.png" alt="Video icon" />
        <p className="text-xl text-[#4CAF50] font-bold">oakpark</p>
      </Link>
      <div className="flex justify-between">
        {canJoinSession && (
          <button className="rounded-2xl">CREATE A SESSION</button>
        )}

        {canCreateSession && (
          <button className="rounded-2xl"> JOIN A SESSION</button>
        )}
      </div>
    </nav>
  );
};

export default Navbar;
