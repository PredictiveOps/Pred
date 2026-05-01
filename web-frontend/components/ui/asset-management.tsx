"use client";

import {
	AlertCircle,
	ArrowUpDown,
	Clock,
	Filter,
	MoreVertical,
	Plus,
	TrendingUp,
} from "lucide-react";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";

interface Asset {
	id: string;
	name: string;
	location: string;
	sensorType: string;
	lastCalibration: string;
	connectionStatus: "ACTIVE" | "OFFLINE";
}

export function AssetManagement() {
	const [assets] = useState<Asset[]>([
		{
			id: "ID: PX-350-0021",
			name: "Thermovision XT-400",
			location: "Bay 4, Sector G",
			sensorType: "THERMAL",
			lastCalibration: "2023-11-14",
			connectionStatus: "ACTIVE",
		},
		{
			id: "ID: PX-3485-0042",
			name: "Vibration Probe MKII",
			location: "Conveyor Line B",
			sensorType: "ACOUSTIC",
			lastCalibration: "2023-10-28",
			connectionStatus: "ACTIVE",
		},
		{
			id: "ID: PX-3582-0031",
			name: "Flow Meter FlowX 2",
			location: "Cooling Unit 02",
			sensorType: "FLUMIC",
			lastCalibration: "CALIBRATION OVERDUE",
			connectionStatus: "OFFLINE",
		},
		{
			id: "ID: PX-3586-7702",
			name: "Power Quality Monitor",
			location: "Substation A",
			sensorType: "ELECTRICAL",
			lastCalibration: "2024-01-05",
			connectionStatus: "ACTIVE",
		},
		{
			id: "ID: PX-3680-2319",
			name: "Pressure Sensor 90k",
			location: "Steam Pipe 14",
			sensorType: "PRESSURE",
			lastCalibration: "2023-12-20",
			connectionStatus: "ACTIVE",
		},
	]);

	const stats = [
		{
			label: "TOTAL ACTIVE ASSETS",
			value: "1,242",
			change: "+12.4%",
			icon: TrendingUp,
			color: "text-blue-400",
		},
		{
			label: "OFFLINE NODES",
			value: "06",
			change: "CRITICAL",
			icon: AlertCircle,
			color: "text-red-400",
		},
		{
			label: "UPCOMING CALIBRATIONS",
			value: "42",
			change: "30 DAYS",
			icon: Clock,
			color: "text-orange-400",
		},
		{
			label: "DATA THROUGHPUT",
			value: "8.2",
			unit: "GB/s",
			change: "PEAK",
			icon: TrendingUp,
			color: "text-green-400",
		},
	];

	return (
		<div className="space-y-6">
			{/* Header */}
			<div className="flex items-start justify-between mb-6">
				<div>
					<h1 className="text-3xl font-bold mb-2">Asset Management</h1>
					<p className="text-slate-400 text-sm">
						Manage and monitor 1,248 connected industrial sensors across 4
						nodes.
					</p>
				</div>
				<Button className="bg-blue-600 hover:bg-blue-700 px-6 font-semibold uppercase text-sm flex items-center gap-2">
					<Plus className="w-4 h-4" />
					ADD NEW ASSET
				</Button>
			</div>

			{/* Stats Cards */}
			<div className="grid grid-cols-4 gap-4">
				{stats.map((stat, idx) => {
					const Icon = stat.icon;
					return (
						<Card key={idx} className="bg-slate-800/50 border-slate-700 p-6">
							<div className="flex justify-between items-start mb-4">
								<div className="text-xs text-slate-400 font-semibold uppercase">
									{stat.label}
								</div>
								<Icon className={`w-4 h-4 ${stat.color}`} />
							</div>
							<div className="mb-2">
								<span className={`text-2xl font-bold ${stat.color}`}>
									{stat.value}
								</span>
								{stat.unit && (
									<span className={`text-xs ${stat.color} ml-1`}>
										{stat.unit}
									</span>
								)}
							</div>
							<div className={`text-xs font-semibold ${stat.color}`}>
								{stat.change}
							</div>
						</Card>
					);
				})}
			</div>

			{/* Asset Repository */}
			<Card className="bg-slate-800/50 border-slate-700 p-6">
				<div className="flex justify-between items-center mb-6">
					<h2 className="text-sm font-semibold text-slate-300 uppercase tracking-wider">
						Asset Repository
					</h2>
					<div className="flex gap-3">
						<Button className="bg-slate-700 hover:bg-slate-600 px-3 py-1 text-xs font-semibold flex items-center gap-2">
							<Filter className="w-3 h-3" />
							Filter By Type
						</Button>
						<Button className="bg-slate-700 hover:bg-slate-600 px-3 py-1 text-xs font-semibold flex items-center gap-2">
							<ArrowUpDown className="w-3 h-3" />
							Sort: Last Calibration
						</Button>
					</div>
				</div>

				{/* Table */}
				<div className="overflow-x-auto">
					<table className="w-full">
						<thead>
							<tr className="border-b border-slate-700">
								<th className="text-xs text-slate-400 font-semibold uppercase text-left py-3 px-4">
									Asset Name / ID
								</th>
								<th className="text-xs text-slate-400 font-semibold uppercase text-left py-3 px-4">
									Location
								</th>
								<th className="text-xs text-slate-400 font-semibold uppercase text-left py-3 px-4">
									Sensor Type
								</th>
								<th className="text-xs text-slate-400 font-semibold uppercase text-left py-3 px-4">
									Last Calibration
								</th>
								<th className="text-xs text-slate-400 font-semibold uppercase text-left py-3 px-4">
									Connection Status
								</th>
								<th className="text-xs text-slate-400 font-semibold uppercase text-center py-3 px-4">
									Actions
								</th>
							</tr>
						</thead>
						<tbody>
							{assets.map((asset, idx) => (
								<tr
									key={idx}
									className="border-b border-slate-700 hover:bg-slate-900/30 transition-colors"
								>
									<td className="py-4 px-4">
										<div className="flex items-center gap-2">
											<div className="w-2 h-2 bg-slate-500 rounded-full"></div>
											<div>
												<div className="text-sm font-semibold text-slate-200">
													{asset.name}
												</div>
												<div className="text-xs text-slate-400">{asset.id}</div>
											</div>
										</div>
									</td>
									<td className="py-4 px-4 text-sm text-slate-300">
										{asset.location}
									</td>
									<td className="py-4 px-4">
										<span className="px-3 py-1 bg-slate-700 text-slate-300 text-xs font-semibold rounded">
											{asset.sensorType}
										</span>
									</td>
									<td className="py-4 px-4 text-sm text-slate-300">
										{asset.lastCalibration === "CALIBRATION OVERDUE" ? (
											<span className="text-red-400 font-semibold">
												{asset.lastCalibration}
											</span>
										) : (
											asset.lastCalibration
										)}
									</td>
									<td className="py-4 px-4">
										<div className="flex items-center gap-2">
											<div
												className={`w-2 h-2 rounded-full ${
													asset.connectionStatus === "ACTIVE"
														? "bg-green-500"
														: "bg-red-500"
												}`}
											></div>
											<span
												className={`text-xs font-semibold ${
													asset.connectionStatus === "ACTIVE"
														? "text-green-400"
														: "text-red-400"
												}`}
											>
												{asset.connectionStatus}
											</span>
										</div>
									</td>
									<td className="py-4 px-4 text-center">
										<button
											type="button"
											className="p-1 hover:bg-slate-700 rounded transition-colors"
										>
											<MoreVertical className="w-4 h-4 text-slate-400" />
										</button>
									</td>
								</tr>
							))}
						</tbody>
					</table>
				</div>
			</Card>
		</div>
	);
}
