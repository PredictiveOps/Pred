"use client";

import {
	AlertTriangle,
	Bell,
	Box,
	FileText,
	Grid3x3,
	HelpCircle,
	Lightbulb,
	List,
	Settings,
} from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import {
	Sidebar,
	SidebarContent,
	SidebarFooter,
	SidebarHeader,
	SidebarProvider,
} from "@/components/ui/sidebar";
import SidebarUserCard from "../sidebar-user-card";

export function LayoutContent({ children }: { children: React.ReactNode }) {
	const pathname = usePathname();

	const isActive = (path: string) => pathname === path;

	const navItems = [
		{ path: "/dashboard", label: "Dashboard", icon: Grid3x3 },
		{ path: "/motors", label: "Motors", icon: Settings },
		{ path: "/alerts", label: "Alerts", icon: AlertTriangle },
		{ path: "/assets", label: "Assets", icon: Box },
		{ path: "/predictions", label: "Predictions", icon: Lightbulb },
		{ path: "/raw-events", label: "Raw Events", icon: List },
	];

	return (
		<SidebarProvider>
			<Sidebar className="!bg-white !text-gray-900 border-r border-gray-200">
				<SidebarHeader className="border-b border-gray-200 pb-4 bg-white">
					<div className="space-y-1">
						<div className="text-lg font-bold text-gray-900">PREDICTIVE-X</div>
						<div className="text-xs text-gray-500">MAINTENANCE HUB</div>
						<div className="text-xs text-gray-500">PRECISION MONITORING</div>
					</div>
				</SidebarHeader>
				<SidebarContent className="pt-4 px-2 bg-white">
					<nav className="space-y-1">
						{navItems.map(({ path, label, icon: Icon }) => (
							<Link
								key={path}
								href={path}
								className={`w-full flex items-center gap-3 px-3 py-2 rounded-md cursor-pointer transition-colors ${
									isActive(path)
										? "bg-blue-50 text-blue-600 border-l-4 border-blue-600"
										: "hover:bg-gray-100 text-gray-600"
								}`}
							>
								<Icon className="w-5 h-5" />
								<span className="text-sm font-medium">{label}</span>
							</Link>
						))}
					</nav>

					<div className="mt-8 pt-4 border-t border-gray-200 space-y-1">
						<button
							type="button"
							className="w-full flex items-center gap-3 px-3 py-2 rounded-md hover:bg-gray-100 cursor-pointer text-gray-600 transition-colors"
						>
							<HelpCircle className="w-5 h-5" />
							<span className="text-sm">Support</span>
						</button>
						<button
							type="button"
							className="w-full flex items-center gap-3 px-3 py-2 rounded-md hover:bg-gray-100 cursor-pointer text-gray-600 transition-colors"
						>
							<FileText className="w-5 h-5" />
							<span className="text-sm">Logs</span>
						</button>
					</div>
				</SidebarContent>
				<SidebarFooter className="border-t border-gray-200 p-3 bg-white">
					<SidebarUserCard />
				</SidebarFooter>
			</Sidebar>

			<div className="flex-1 bg-gray-50 text-gray-900 min-h-screen">
				{/* Header */}
				<div className="border-b border-gray-200 bg-white/80 backdrop-blur flex">
					<div className="px-6 py-4 items-center w-full">
						<div className="flex ml-auto items-center gap-4 w-fit">
							<div className="text-xs text-gray-500">
								SYSTEM STATUS:{" "}
								<span className="text-green-600 font-semibold">OPTIMAL</span>
							</div>
							<Bell className="w-5 h-5 text-gray-400" />
							<Settings className="w-5 h-5 text-gray-400" />
						</div>
					</div>
				</div>

				{/* Main Content */}
				<div className="p-6">{children}</div>
			</div>
		</SidebarProvider>
	);
}
