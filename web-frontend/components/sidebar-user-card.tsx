import { User } from "lucide-react";
import { useSession } from "next-auth/react";

export default function SidebarUserCard() {
	const { data: session } = useSession();

	return (
		<div className="grid grid-cols-[auto_1fr] grid-rows-2 gap-x-3 items-center">
			<div className="row-span-2 flex items-center justify-center w-9 h-9 rounded-full bg-gray-200">
				<User className="w-5 h-5 text-gray-500" />
			</div>
			<span className="text-sm font-medium text-gray-900 leading-tight truncate">
				{session?.user?.name ?? "—"}
			</span>
			<span className="text-xs text-gray-500 leading-tight truncate">
				{session?.user?.email ?? "—"}
			</span>
		</div>
	);
}
