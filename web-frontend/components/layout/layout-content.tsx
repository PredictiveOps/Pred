"use client";

import {
	AlertTriangle,
	Bell,
	Box,
	FileText,
	Grid3x3,
	HelpCircle,
	Lightbulb,
	Settings,
} from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import {
	Sidebar,
	SidebarContent,
	SidebarHeader,
	SidebarProvider,
} from "@/components/ui/sidebar";

export function LayoutContent({ children }: { children: React.ReactNode }) {
	const pathname = usePathname();

	const isActive = (path: string) => pathname === path;

	const navItems = [
		{ path: "/dashboard", label: "Dashboard", icon: Grid3x3 },
		{ path: "/motors", label: "Motors", icon: Settings },
		{ path: "/alerts", label: "Alerts", icon: AlertTriangle },
		{ path: "/assets", label: "Assets", icon: Box },
		{ path: "/predictions", label: "Predictions", icon: Lightbulb },
	];

	return (
		<SidebarProvider>
			<Sidebar className="!bg-slate-900 !text-white border-r border-slate-700">
				<SidebarHeader className="border-b border-slate-700 pb-4 bg-slate-900">
					<div className="space-y-1">
						<div className="text-lg font-bold text-white">PREDICTIVE-X</div>
						<div className="text-xs text-slate-400">MAINTENANCE HUB</div>
						<div className="text-xs text-slate-400">PRECISION MONITORING</div>
					</div>
				</SidebarHeader>
				<SidebarContent className="pt-4 px-2 bg-slate-900">
					<nav className="space-y-1">
						{navItems.map(({ path, label, icon: Icon }) => (
							<Link
								key={path}
								href={path}
								className={`w-full flex items-center gap-3 px-3 py-2 rounded-md cursor-pointer transition-colors ${
									isActive(path)
										? "bg-slate-800 text-blue-400 border-l-4 border-blue-400"
										: "hover:bg-slate-800 text-slate-300"
								}`}
							>
								<Icon className="w-5 h-5" />
								<span className="text-sm font-medium">{label}</span>
							</Link>
						))}
					</nav>

					<div className="mt-8 pt-4 border-t border-slate-700 space-y-1">
						<button
							type="button"
							className="w-full flex items-center gap-3 px-3 py-2 rounded-md hover:bg-slate-800 cursor-pointer text-slate-300 transition-colors"
						>
							<HelpCircle className="w-5 h-5" />
							<span className="text-sm">Support</span>
						</button>
						<button
							type="button"
							className="w-full flex items-center gap-3 px-3 py-2 rounded-md hover:bg-slate-800 cursor-pointer text-slate-300 transition-colors"
						>
							<FileText className="w-5 h-5" />
							<span className="text-sm">Logs</span>
						</button>
					</div>
				</SidebarContent>
			</Sidebar>

			<div className="flex-1 bg-slate-950 text-white min-h-screen">
				{/* Header */}
				<div className="border-b border-slate-800 bg-slate-900/50 backdrop-blur flex">
					<div className="px-6 py-4 items-center w-full">
						<div className="flex ml-auto items-center gap-4 w-fit">
							<div className="text-xs text-slate-400">
								SYSTEM STATUS:{" "}
								<span className="text-green-400 font-semibold">OPTIMAL</span>
							</div>
							<Bell className="w-5 h-5 text-slate-400" />
							<Settings className="w-5 h-5 text-slate-400" />
						</div>
					</div>
				</div>

				{/* Main Content */}
				<div className="p-6">{children}</div>
			</div>
		</SidebarProvider>
	);
}
