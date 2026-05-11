"use client";

import { AlertTriangle, Download, Filter } from "lucide-react";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";

interface HighRiskAsset {
	id: string;
	location: string;
	anomalyScore: number;
	estimatedFailure: string;
	status: "CRITICAL FAILURE" | "DEGRADING" | "OBSERVATION" | "OPERATIONAL";
}

export function PredictionsDashboard() {
	const [assets] = useState<HighRiskAsset[]>([
		{
			id: "TRB-402-G1",
			location: "North Sector - Bay 4",
			anomalyScore: 92,
			estimatedFailure: "14.2 Hours",
			status: "CRITICAL FAILURE",
		},
		{
			id: "MOT-119-X",
			location: "Conveyor Line C",
			anomalyScore: 78,
			estimatedFailure: "5 Days",
			status: "DEGRADING",
		},
		{
			id: "PMP-SS-IM1",
			location: "Cooling System West",
			anomalyScore: 65,
			estimatedFailure: "5 Days",
			status: "OBSERVATION",
		},
		{
			id: "SRV-002-HUB",
			location: "Control Room Annex",
			anomalyScore: 42,
			estimatedFailure: "12 Days",
			status: "OPERATIONAL",
		},
	]);

	const getStatusColor = (status: string) => {
		switch (status) {
			case "CRITICAL FAILURE":
				return "bg-red-50 text-red-600 border border-red-200";
			case "DEGRADING":
				return "bg-orange-50 text-orange-600 border border-orange-200";
			case "OBSERVATION":
				return "bg-gray-100 text-gray-600 border border-gray-200";
			case "OPERATIONAL":
				return "bg-blue-50 text-blue-600 border border-blue-200";
			default:
				return "bg-gray-100 text-gray-500";
		}
	};

	return (
		<div className="space-y-6">
			{/* Header */}
			<div className="flex items-start justify-between mb-6">
				<div>
					<h1 className="text-3xl font-bold mb-2">Predictions Dashboard</h1>
					<p className="text-gray-500 text-sm">
						Real-time asset telemetry and failure probability modeling.
					</p>
				</div>
				<div className="flex gap-3">
					<Button className="bg-gray-100 hover:bg-gray-200 text-gray-700 px-4 font-semibold uppercase text-sm flex items-center gap-2">
						<Filter className="w-4 h-4" />
						FILTER VIEW
					</Button>
					<Button className="bg-blue-600 hover:bg-blue-700 text-white px-4 font-semibold uppercase text-sm flex items-center gap-2">
						<Download className="w-4 h-4" />
						EXPORT REPORT
					</Button>
				</div>
			</div>

			{/* Charts Grid */}
			<div className="grid grid-cols-3 gap-6">
				{/* Anomaly Scores Chart - Takes 2 columns */}
				<div className="col-span-2">
					<Card className="bg-white border-gray-200 p-6">
						<div className="flex justify-between items-start mb-6">
							<div>
								<h2 className="text-sm font-semibold text-gray-700 uppercase tracking-wider">
									Anomaly Scores: Plant-Wide Trend
								</h2>
							</div>
							<div className="text-xs text-blue-600 font-semibold">
								INTERVAL: 30D
							</div>
						</div>

						{/* Chart */}
						<div className="h-56 flex items-end justify-around gap-1 mb-6 px-4">
							{[
								45, 52, 38, 61, 48, 56, 42, 58, 51, 47, 72, 68, 65, 42, 55, 48,
								61, 58, 65, 52, 48, 55, 42, 68, 58, 62, 48, 55, 72, 65, 58,
							].map((height, idx) => (
								<div
									key={idx}
									className="flex-1 bg-gray-200 rounded hover:bg-blue-500 transition-colors"
									style={{ height: `${height}%` }}
								></div>
							))}
						</div>

						{/* Timeline Labels */}
						<div className="flex justify-between text-xs text-gray-500 px-2">
							<span>OCT 01</span>
							<span>OCT 10</span>
							<span>OCT 20</span>
							<span>OCT 31</span>
						</div>
					</Card>
				</div>

				{/* Failure Matrix */}
				<div>
					<Card className="bg-white border-gray-200 p-6 h-full flex flex-col">
						<h2 className="text-sm font-semibold text-gray-700 uppercase tracking-wider mb-6">
							Failure Matrix
						</h2>

						{/* Scatter Plot */}
						<div className="flex-1 bg-gray-50 rounded mb-6 relative p-4 flex items-center justify-center">
							<svg
								className="w-full h-full"
								viewBox="0 0 200 200"
								preserveAspectRatio="none"
								role="img"
								aria-label="Failure matrix scatter plot"
							>
								{/* Grid lines */}
								<line
									x1="0"
									y1="50%"
									x2="100%"
									y2="50%"
									stroke="#d1d5db"
									strokeWidth="0.5"
									opacity="0.8"
								/>
								<line
									x1="50%"
									y1="0"
									x2="50%"
									y2="100%"
									stroke="#d1d5db"
									strokeWidth="0.5"
									opacity="0.8"
								/>

								{/* Data points - Blue and Orange dots */}
								<circle cx="30" cy="40" r="3" fill="#3b82f6" />
								<circle cx="60" cy="70" r="3" fill="#3b82f6" />
								<circle cx="80" cy="60" r="3" fill="#f97316" />
								<circle cx="120" cy="80" r="3" fill="#f97316" />
								<circle cx="140" cy="90" r="3" fill="#f97316" />
								<circle cx="160" cy="75" r="3" fill="#ef4444" />
								<circle cx="170" cy="85" r="3" fill="#ef4444" />
							</svg>

							{/* Axis Labels */}
							<div className="absolute bottom-1 left-1/2 -translate-x-1/2 text-xs text-gray-500 font-semibold">
								TIME TO FAILURE (D)
							</div>
							<div className="absolute left-1 top-1/2 -translate-y-1/2 text-xs text-gray-500 font-semibold transform -rotate-90 origin-left">
								PROBABILITY
							</div>
						</div>

						{/* Legend */}
						<div className="space-y-2 text-xs">
							<div className="flex justify-between">
								<span className="text-gray-500">Critical Cluster:</span>
								<span className="text-orange-500 font-semibold">
									4 Assets Found
								</span>
							</div>
							<div className="flex justify-between">
								<span className="text-gray-500">Stable Assets:</span>
								<span className="text-blue-600 font-semibold">162 Total</span>
							</div>
						</div>
					</Card>
				</div>
			</div>

			{/* High-Risk Assets Table */}
			<Card className="bg-white border-gray-200 p-6">
				<div className="flex items-center gap-3 mb-6">
					<AlertTriangle className="w-5 h-5 text-red-500" />
					<h2 className="text-sm font-semibold text-gray-700 uppercase tracking-wider">
						Priority 1: High-Risk Assets
					</h2>
					<span className="ml-auto text-xs font-semibold text-red-600 bg-red-50 px-3 py-1 rounded border border-red-200">
						4 DETECTED ANOMALIES
					</span>
				</div>

				{/* Table */}
				<div className="overflow-x-auto">
					<table className="w-full">
						<thead>
							<tr className="border-b border-gray-200">
								<th className="text-xs text-gray-500 font-semibold uppercase text-left py-3 px-4">
									Asset ID
								</th>
								<th className="text-xs text-gray-500 font-semibold uppercase text-left py-3 px-4">
									Location
								</th>
								<th className="text-xs text-gray-500 font-semibold uppercase text-left py-3 px-4">
									Anomaly Score
								</th>
								<th className="text-xs text-gray-500 font-semibold uppercase text-left py-3 px-4">
									Est. Failure
								</th>
								<th className="text-xs text-gray-500 font-semibold uppercase text-left py-3 px-4">
									Status
								</th>
								<th className="text-xs text-gray-500 font-semibold uppercase text-center py-3 px-4">
									Actions
								</th>
							</tr>
						</thead>
						<tbody>
							{assets.map((asset, idx) => (
								<tr
									key={idx}
									className="border-b border-gray-100 hover:bg-gray-50 transition-colors"
								>
									<td className="py-4 px-4 text-sm font-semibold text-gray-800">
										{asset.id}
									</td>
									<td className="py-4 px-4 text-sm text-gray-600">
										{asset.location}
									</td>
									<td className="py-4 px-4">
										<div className="flex items-center gap-2">
											<div className="w-24 bg-gray-200 rounded-full h-1.5 overflow-hidden">
												<div
													className={`h-full rounded-full transition-all ${
														asset.anomalyScore > 80
															? "bg-red-500"
															: asset.anomalyScore > 60
																? "bg-orange-500"
																: "bg-yellow-500"
													}`}
													style={{ width: `${asset.anomalyScore}%` }}
												></div>
											</div>
											<span className="text-xs font-semibold text-gray-600 w-8">
												{asset.anomalyScore}%
											</span>
										</div>
									</td>
									<td className="py-4 px-4 text-sm text-gray-600">
										{asset.estimatedFailure}
									</td>
									<td className="py-4 px-4">
										<span
											className={`px-3 py-1 text-xs font-semibold rounded ${getStatusColor(
												asset.status,
											)}`}
										>
											{asset.status}
										</span>
									</td>
									<td className="py-4 px-4 text-center">
										<button
											type="button"
											className="text-blue-600 hover:text-blue-800 text-xs font-semibold uppercase transition-colors"
										>
											Investigate
										</button>
									</td>
								</tr>
							))}
						</tbody>
					</table>
				</div>
			</Card>

			{/* Bottom Metrics */}
			<div className="grid grid-cols-3 gap-6">
				<Card className="bg-white border-gray-200 p-4">
					<div className="text-xs text-gray-500 font-semibold uppercase mb-3">
						Long-term Health Index
					</div>
					<div className="text-2xl font-bold text-blue-600">78.5%</div>
					<div className="text-xs text-gray-400 mt-2">+2.3% from last week</div>
				</Card>
				<Card className="bg-white border-gray-200 p-4">
					<div className="text-xs text-gray-500 font-semibold uppercase mb-3">
						Predictive Accuracy
					</div>
					<div className="text-2xl font-bold text-green-600">94.2%</div>
					<div className="text-xs text-gray-400 mt-2">Based on 12M records</div>
				</Card>
				<Card className="bg-white border-gray-200 p-4">
					<div className="text-xs text-gray-500 font-semibold uppercase mb-3">
						Next Scheduled Sync
					</div>
					<div className="text-2xl font-bold text-orange-500">2h 34m</div>
					<div className="text-xs text-gray-400 mt-2">UTC+0</div>
				</Card>
			</div>
		</div>
	);
}
