import { LogOut, User } from "lucide-react";
import { signOut, useSession } from "next-auth/react";

export default function SidebarUserCard() {
	const { data: session } = useSession();

	return (
		<div className="grid grid-cols-[auto_1fr_auto] grid-rows-2 gap-x-3 items-center">
			<div className="row-span-2 flex items-center justify-center w-9 h-9 rounded-full bg-gray-200">
				<User className="w-5 h-5 text-gray-500" />
			</div>
			<span className="col-start-2 text-sm font-medium text-gray-900 leading-tight truncate">
				{session?.user?.name ?? "—"}
			</span>
			<span className="col-start-2 text-xs text-gray-500 leading-tight truncate">
				{session?.user?.email ?? "—"}
			</span>
			<button
				type="button"
				onClick={() => signOut({ callbackUrl: "/login" })}
				className="row-span-2 flex items-center justify-center w-8 h-8 rounded-md text-gray-500 hover:bg-gray-100 hover:text-gray-900 row-start-1 row-span-2 col-start-3"
				aria-label="Sign out"
				title="Sign out"
			>
				<LogOut className="w-4 h-4" />
			</button>
		</div>
	);
}
