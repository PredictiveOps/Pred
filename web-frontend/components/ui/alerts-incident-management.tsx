"use client";

import {
	AlertTriangle,
	ChevronLeft,
	ChevronRight,
	Download,
	RefreshCw,
} from "lucide-react";
import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";

interface Alert {
	id: string;
	asset: string;
	severity: "CRITICAL" | "WARNING" | "INFO";
	type: string;
	timeDetected: string;
	status: string;
	acknowledged: boolean;
	description: string;
}

const initialMockAlerts: Alert[] = [
	{
		id: "#71630",
		asset: "TURB-A-09",
		severity: "CRITICAL",
		type: "Vibration Multi-Factor Over-Limit",
		timeDetected: "2025-N1-02:30",
		status: "UNACKNOWLEDGED",
		acknowledged: false,
		description: "Vibration levels exceeded safe operating parameters",
	},
	{
		id: "#71636",
		asset: "HVAC-M-01",
		severity: "WARNING",
		type: "Thermal Gradient Anomaly",
		timeDetected: "2025-10-02",
		status: "ACKNOWLEDGED",
		acknowledged: true,
		description: "Temperature gradient detected beyond expected range",
	},
	{
		id: "#71625",
		asset: "PUMP-R-04",
		severity: "CRITICAL",
		type: "Lubrication Pressure Drop",
		timeDetected: "2025-10-02 01:44:00",
		status: "UNACKNOWLEDGED",
		acknowledged: false,
		description: "Oil pressure dropped below minimum threshold",
	},
	{
		id: "#74770",
		asset: "COMPRESSOR-04",
		severity: "INFO",
		type: "Scheduled Maintenance Required",
		timeDetected: "2025-10-01 11:30:00",
		status: "RESCUED",
		acknowledged: true,
		description: "Equipment requires periodic maintenance",
	},
	{
		id: "#74770-B",
		asset: "PUMP-R-02",
		severity: "WARNING",
		type: "Cavitation Noise Detected",
		timeDetected: "2025-10-01 09:16:22",
		status: "ACKNOWLEDGED",
		acknowledged: true,
		description: "Unusual cavitation noise pattern detected",
	},
];

// Potential error scenarios for assets
const errorScenarios = [
	{
		asset: "TURB-A-09",
		severity: "CRITICAL" as const,
		type: "Rotor Imbalance Detected",
		description: "Rotor imbalance levels exceeded operational limits",
	},
	{
		asset: "HVAC-M-01",
		severity: "WARNING" as const,
		type: "Coolant Flow Rate Low",
		description: "Coolant circulation below minimum threshold",
	},
	{
		asset: "PUMP-R-04",
		severity: "CRITICAL" as const,
		type: "Seal Integrity Compromised",
		description: "Pump seal showing signs of degradation",
	},
	{
		asset: "PUMP-R-02",
		severity: "WARNING" as const,
		type: "Pressure Fluctuation Detected",
		description: "Abnormal pressure oscillations in outlet line",
	},
	{
		asset: "COMPRESSOR-04",
		severity: "WARNING" as const,
		type: "Temperature Rise Alert",
		description: "Compressor temperature exceeding normal range",
	},
];

const getSeverityColor = (severity: string) => {
	switch (severity) {
		case "CRITICAL":
			return "text-red-500";
		case "WARNING":
			return "text-orange-500";
		case "INFO":
			return "text-blue-500";
		default:
			return "text-gray-500";
	}
};

const getSeverityBgColor = (severity: string) => {
	switch (severity) {
		case "CRITICAL":
			return "bg-red-500/10 text-red-500";
		case "WARNING":
			return "bg-orange-500/10 text-orange-500";
		case "INFO":
			return "bg-blue-500/10 text-blue-500";
		default:
			return "bg-gray-500/10 text-gray-500";
	}
};

const getFormattedTime = () => {
	const now = new Date();
	const year = now.getFullYear();
	const month = String(now.getMonth() + 1).padStart(2, "0");
	const day = String(now.getDate()).padStart(2, "0");
	const hours = String(now.getHours()).padStart(2, "0");
	const minutes = String(now.getMinutes()).padStart(2, "0");
	const seconds = String(now.getSeconds()).padStart(2, "0");
	return `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`;
};

export function AlertsIncidentManagement() {
	const [allAlerts, setAllAlerts] = useState<Alert[]>(initialMockAlerts);
	const [selectedAlert, setSelectedAlert] = useState<Alert>(
		initialMockAlerts[0],
	);
	const [severityFilter, setSeverityFilter] = useState<string | null>(null);
	const [assetGroupFilter, setAssetGroupFilter] = useState("ALL ASSETS");
	const [timeRange, setTimeRange] = useState("LAST 24 HOURS");
	const [currentPage, setCurrentPage] = useState(1);
	const [acknowledgedAlerts, setAcknowledgedAlerts] = useState<Set<string>>(
		new Set(initialMockAlerts.filter((a) => a.acknowledged).map((a) => a.id)),
	);
	const [nextAlertId, setNextAlertId] = useState(100);
	const [autoGenerateEnabled, setAutoGenerateEnabled] = useState(true);

	// Auto-generate new alerts periodically (every 15 seconds)
	useEffect(() => {
		if (!autoGenerateEnabled) return;

		const interval = setInterval(() => {
			// Randomly decide if a new error should occur (30% chance)
			if (Math.random() > 0.7) {
				const randomError =
					errorScenarios[Math.floor(Math.random() * errorScenarios.length)];
				const newAlertId = `#${Date.now().toString().slice(-5)}${nextAlertId}`;

				const newAlert: Alert = {
					id: newAlertId,
					asset: randomError.asset,
					severity: randomError.severity,
					type: randomError.type,
					timeDetected: getFormattedTime(),
					status: "UNACKNOWLEDGED",
					acknowledged: false,
					description: randomError.description,
				};

				setAllAlerts((prev) => [newAlert, ...prev]);
				setNextAlertId((prev) => prev + 1);

				// Auto-select the new alert
				setSelectedAlert(newAlert);
			}
		}, 15000); // Check every 15 seconds

		return () => clearInterval(interval);
	}, [autoGenerateEnabled, nextAlertId]);

	// Filter alerts based on severity filter
	const filteredAlerts = severityFilter
		? allAlerts.filter((alert) => alert.severity === severityFilter)
		: allAlerts;

	const itemsPerPage = 5;
	const totalPages = Math.ceil(filteredAlerts.length / itemsPerPage);

	const unacknowledgedCount = allAlerts.filter(
		(alert) => !acknowledgedAlerts.has(alert.id),
	).length;
	const criticalCount = allAlerts.filter(
		(alert) =>
			alert.severity === "CRITICAL" && !acknowledgedAlerts.has(alert.id),
	).length;
	const warningCount = allAlerts.filter(
		(alert) =>
			alert.severity === "WARNING" && !acknowledgedAlerts.has(alert.id),
	).length;
	const infoCount = allAlerts.filter(
		(alert) => alert.severity === "INFO" && !acknowledgedAlerts.has(alert.id),
	).length;

	const handleAcknowledgeAlert = (alertId: string) => {
		const newAcknowledged = new Set(acknowledgedAlerts);
		newAcknowledged.add(alertId);
		setAcknowledgedAlerts(newAcknowledged);
	};

	const handleAcknowledgeAll = () => {
		const newAcknowledged = new Set(acknowledgedAlerts);
		filteredAlerts.forEach((alert) => {
			newAcknowledged.add(alert.id);
		});
		setAcknowledgedAlerts(newAcknowledged);
	};

	return (
		<div className="space-y-6">
			{/* Header */}
			<div className="flex justify-between items-start">
				<div>
					<h2 className="text-2xl font-bold mb-1">
						ALERTS & INCIDENT MANAGEMENT
					</h2>
					<p className="text-slate-400 text-sm">
						Active Telemetry Anomalies and Fault Records ({unacknowledgedCount}{" "}
						unacknowledged)
					</p>
				</div>
				<div className="flex gap-2">
					<Button
						onClick={() => setAutoGenerateEnabled(!autoGenerateEnabled)}
						className={`gap-2 font-semibold text-xs ${
							autoGenerateEnabled
								? "bg-green-600 hover:bg-green-700"
								: "bg-slate-700 hover:bg-slate-600"
						}`}
					>
						<RefreshCw className="w-4 h-4" />
						{autoGenerateEnabled ? "LIVE" : "PAUSED"}
					</Button>
					<Button className="bg-blue-600 hover:bg-blue-700 gap-2">
						<Download className="w-4 h-4" />
						EXPORT JSON
					</Button>
				</div>
			</div>

			{/* Advanced Filtering Section */}
			<div className="bg-slate-800/30 border border-slate-700 rounded px-4 py-3 space-y-3">
				<div className="grid grid-cols-4 gap-4 items-center">
					{/* Severity Filter */}
					<div>
						<p className="text-xs text-slate-400 font-semibold block mb-2">
							FILTER BY SEVERITY:
						</p>
						<div className="flex gap-2">
							{[
								{ label: "CRITICAL", value: "CRITICAL", count: criticalCount },
								{ label: "WARNING", value: "WARNING", count: warningCount },
								{ label: "INFO", value: "INFO", count: infoCount },
							].map((tab) => {
								const isActive = severityFilter === tab.label;
								return (
									<Button
										key={tab.label}
										onClick={() =>
											setSeverityFilter(isActive ? null : tab.label)
										}
										className={`px-4 py-2 text-sm font-medium transition-colors ${
											isActive
												? "text-orange-400 border-b-2 border-orange-400"
												: "text-slate-400 hover:text-slate-300"
										}`}
									>
										{tab.label} ({tab.count})
									</Button>
								);
							})}
						</div>
					</div>

					{/* Asset Group Filter */}
					<div>
						<p className="text-xs text-slate-400 font-semibold block mb-2">
							ASSET GROUP:
						</p>
						<select
							value={assetGroupFilter}
							onChange={(e) => setAssetGroupFilter(e.target.value)}
							className="w-full bg-slate-700 border border-slate-600 rounded px-3 py-1 text-xs text-slate-300 focus:outline-none focus:border-blue-500"
						>
							<option>ALL ASSETS</option>
							<option>TURBINES</option>
							<option>PUMPS</option>
							<option>COMPRESSORS</option>
							<option>HVAC SYSTEMS</option>
						</select>
					</div>

					{/* Time Range Filter */}
					<div>
						<p className="text-xs text-slate-400 font-semibold block mb-2">
							TIME RANGE:
						</p>
						<select
							value={timeRange}
							onChange={(e) => setTimeRange(e.target.value)}
							className="w-full bg-slate-700 border border-slate-600 rounded px-3 py-1 text-xs text-slate-300 focus:outline-none focus:border-blue-500"
						>
							<option>LAST 24 HOURS</option>
							<option>LAST 7 DAYS</option>
							<option>LAST 30 DAYS</option>
							<option>LAST 90 DAYS</option>
						</select>
					</div>

					{/* Acknowledge All Button */}
					<div className="flex justify-end">
						<Button
							onClick={handleAcknowledgeAll}
							className="bg-green-600 hover:bg-green-700 text-white font-semibold text-xs"
						>
							✓ ACKNOWLEDGE ALL
						</Button>
					</div>
				</div>
			</div>

			{/* Filter Tabs */}
			<div className="flex gap-2 border-b border-slate-700 pb-4">
				{[
					{ label: "CRITICAL", count: criticalCount },
					{ label: "WARNING", count: warningCount },
					{ label: "INFO", count: infoCount },
				].map((tab) => (
					<button
						key={tab.label}
						className={`px-4 py-2 text-sm font-medium transition-colors ${
							tab.label === "CRITICAL"
								? "text-orange-400 border-b-2 border-orange-400"
								: "text-slate-400 hover:text-slate-300"
						}`}
					>
						{tab.label} ({tab.count})
					</button>
				))}
			</div>

			<div className="grid grid-cols-3 gap-6">
				{/* Incident Trend Chart */}
				<Card className="bg-slate-800/50 border-slate-700 p-4">
					<h3 className="text-sm font-semibold mb-4 text-slate-300">
						INCIDENT TREND
					</h3>
					<div className="space-y-4">
						<div className="flex items-end justify-around h-32 gap-2">
							<div className="flex flex-col items-center gap-2">
								<div
									className="w-8 bg-blue-500/40 rounded"
									style={{ height: "60px" }}
								></div>
								<span className="text-xs text-slate-400">00:00</span>
							</div>
							<div className="flex flex-col items-center gap-2">
								<div
									className="w-8 bg-blue-500 rounded"
									style={{ height: "80px" }}
								></div>
								<span className="text-xs text-slate-400">12:00</span>
							</div>
							<div className="flex flex-col items-center gap-2">
								<div
									className="w-8 bg-orange-500 rounded"
									style={{ height: "100px" }}
								></div>
								<span className="text-xs text-slate-400">24:00</span>
							</div>
						</div>
						<div className="flex justify-around pt-2 border-t border-slate-700">
							<div className="text-center">
								<div className="text-xs text-slate-400">00:00</div>
								<div className="text-sm font-semibold">5</div>
							</div>
							<div className="text-center">
								<div className="text-xs text-slate-400">12:00</div>
								<div className="text-sm font-semibold">8</div>
							</div>
							<div className="text-center">
								<div className="text-xs text-slate-400">24:00</div>
								<div className="text-sm font-semibold">12</div>
							</div>
						</div>
					</div>
				</Card>

				{/* Asset Health Status */}
				<Card className="bg-slate-800/50 border-slate-700 p-4">
					<h3 className="text-sm font-semibold mb-4 text-slate-300">
						ASSET HEALTH STATUS
					</h3>
					<div className="space-y-3">
						{[
							{ name: "TURB-A-09", status: "CRITICAL", percentage: 85 },
							{ name: "HVAC-M-01", status: "WARM", percentage: 65 },
							{ name: "PUMP-R-04", status: "CRITICAL", percentage: 90 },
							{ name: "PUMP-R-02", status: "NORMAL", percentage: 45 },
						].map((asset) => (
							<div key={asset.name} className="space-y-1">
								<div className="flex justify-between items-center">
									<span className="text-xs font-medium text-slate-300">
										{asset.name}
									</span>
									<span className="text-xs text-slate-400">{asset.status}</span>
								</div>
								<div className="w-full bg-slate-700 rounded-full h-2 overflow-hidden">
									<div
										className={`h-full rounded-full ${
											asset.status === "CRITICAL"
												? "bg-red-500"
												: asset.status === "WARM"
													? "bg-orange-500"
													: "bg-blue-500"
										}`}
										style={{ width: `${asset.percentage}%` }}
									></div>
								</div>
							</div>
						))}
					</div>
				</Card>

				{/* Key Metrics */}
				<Card className="bg-slate-800/50 border-slate-700 p-4 space-y-4">
					<div className="space-y-2">
						<div className="text-xs text-slate-400 font-medium">
							ACTIVE INCIDENTS
						</div>
						<div className="text-2xl font-bold text-orange-400">12</div>
						<div className="text-xs text-slate-400">↑ 2 from last hour</div>
					</div>
					<div className="border-t border-slate-700 pt-4 space-y-2">
						<div className="text-xs text-slate-400 font-medium">
							RESPONSE TIME
						</div>
						<div className="text-2xl font-bold text-blue-400">4.2 min</div>
						<div className="text-xs text-slate-400">
							Average detection to alert
						</div>
					</div>
					<div className="border-t border-slate-700 pt-4 space-y-2">
						<div className="text-xs text-slate-400 font-medium">
							CRITICAL ASSETS
						</div>
						<div className="text-2xl font-bold text-red-400">3</div>
						<div className="text-xs text-slate-400">
							Requiring immediate attention
						</div>
					</div>
				</Card>
			</div>

			{/* Alerts Table */}
			<Card className="bg-slate-800/50 border-slate-700 overflow-hidden">
				<div className="overflow-x-auto">
					<table className="w-full text-sm">
						<thead className="border-b border-slate-700 bg-slate-900/50">
							<tr>
								<th className="px-4 py-3 text-left font-semibold text-slate-300">
									ID
								</th>
								<th className="px-4 py-3 text-left font-semibold text-slate-300">
									ASSET
								</th>
								<th className="px-4 py-3 text-left font-semibold text-slate-300">
									SEVERITY
								</th>
								<th className="px-4 py-3 text-left font-semibold text-slate-300">
									ALERT TYPE
								</th>
								<th className="px-4 py-3 text-left font-semibold text-slate-300">
									TIME DETECTED
								</th>
								<th className="px-4 py-3 text-left font-semibold text-slate-300">
									STATUS
								</th>
								<th className="px-4 py-3 text-left font-semibold text-slate-300">
									ACTIONS
								</th>
							</tr>
						</thead>
						<tbody>
							{filteredAlerts
								.slice(
									(currentPage - 1) * itemsPerPage,
									currentPage * itemsPerPage,
								)
								.map((alert) => {
									const isAcknowledged = acknowledgedAlerts.has(alert.id);
									return (
										<tr
											key={alert.id}
											onClick={() => setSelectedAlert(alert)}
											className={`border-b border-slate-700 hover:bg-slate-700/30 cursor-pointer transition-colors ${
												selectedAlert.id === alert.id ? "bg-slate-700/50" : ""
											} ${isAcknowledged ? "opacity-60" : ""}`}
										>
											<td className="px-4 py-3 font-mono text-xs text-slate-300">
												{alert.id}
											</td>
											<td className="px-4 py-3 font-mono text-xs text-slate-300">
												{alert.asset}
											</td>
											<td className="px-4 py-3">
												<span
													className={`text-xs font-semibold ${getSeverityColor(alert.severity)}`}
												>
													{alert.severity}
												</span>
											</td>
											<td className="px-4 py-3 text-xs text-slate-300">
												{alert.type}
											</td>
											<td className="px-4 py-3 font-mono text-xs text-slate-400">
												{alert.timeDetected}
											</td>
											<td className="px-4 py-3">
												<div className="flex items-center gap-1">
													<div
														className={`w-2 h-2 rounded-full ${
															isAcknowledged ? "bg-green-500" : "bg-red-500"
														}`}
													></div>
													<span
														className={`text-xs font-semibold ${
															isAcknowledged ? "text-green-500" : "text-red-500"
														}`}
													>
														{isAcknowledged ? "ACKNOWLEDGED" : "UNACKNOWLEDGED"}
													</span>
												</div>
											</td>
											<td className="px-4 py-3">
												{!isAcknowledged && (
													<button
														onClick={(e) => {
															e.stopPropagation();
															handleAcknowledgeAlert(alert.id);
														}}
														className="text-blue-400 hover:text-blue-300 text-xs font-semibold"
													>
														ACKNOWLEDGE
													</button>
												)}
											</td>
										</tr>
									);
								})}
						</tbody>
					</table>
				</div>

				{/* Pagination */}
				<div className="flex justify-between items-center px-4 py-3 border-t border-slate-700 bg-slate-900/50">
					<span className="text-xs text-slate-400">
						DISPLAYING {(currentPage - 1) * itemsPerPage + 1} OF{" "}
						{filteredAlerts.length} INCIDENTS
					</span>
					<div className="flex gap-2">
						<button
							onClick={() => setCurrentPage(Math.max(1, currentPage - 1))}
							disabled={currentPage === 1}
							className="p-1 hover:bg-slate-700 disabled:opacity-50 rounded"
						>
							<ChevronLeft className="w-4 h-4" />
						</button>
						<span className="text-xs text-slate-400 px-2 flex items-center">
							{currentPage}
						</span>
						<button
							onClick={() =>
								setCurrentPage(Math.min(totalPages, currentPage + 1))
							}
							disabled={currentPage === totalPages}
							className="p-1 hover:bg-slate-700 disabled:opacity-50 rounded"
						>
							<ChevronRight className="w-4 h-4" />
						</button>
					</div>
				</div>
			</Card>

			<div className="grid grid-cols-2 gap-6">
				{/* Selected Incident Detail */}
				<Card className="bg-slate-800/50 border-slate-700 p-4">
					<div className="space-y-4">
						<div>
							<h3 className="text-sm font-semibold text-slate-300 mb-2">
								SELECTED INCIDENT DETAIL
							</h3>
							<h4 className="text-lg font-bold mb-1">
								{selectedAlert.id} FAILURE ANALYSIS
							</h4>
							<p className="text-xs text-slate-400 mb-2">
								ESTIMATED TIME TO CRITICAL FAILURE: 4h 10m
							</p>
						</div>

						<div className="bg-red-500/10 border border-red-500/30 rounded p-3">
							<div className="flex gap-2">
								<AlertTriangle className="w-4 h-4 text-red-500 flex-shrink-0 mt-0.5" />
								<div className="space-y-1">
									<p className="text-xs font-semibold text-red-400">
										RECOMMENDED ACTION
									</p>
									<p className="text-xs text-slate-300">
										INITIATE EMERGENCY COOLING CYCLE & DECREASE LOAD TO 15%.
										IMMEDIATELY.
									</p>
								</div>
							</div>
						</div>

						<Button className="w-full bg-red-600/80 hover:bg-red-700 text-white font-semibold">
							EMERGENCY SHUTDOWN
						</Button>

						<Button className="w-full bg-slate-700 hover:bg-slate-600 text-white font-semibold">
							VIEW TELEMETRY
						</Button>
					</div>
				</Card>

				{/* Maintenance History & Comments */}
				<Card className="bg-slate-800/50 border-slate-700 p-4">
					<h3 className="text-sm font-semibold text-slate-300 mb-4">
						MAINTENANCE HISTORY & COMMENTS
					</h3>
					<div className="space-y-3 max-h-64 overflow-y-auto">
						{[
							{
								user: "ENG_J_OBRALDI",
								time: "2025-03-04",
								comment:
									"Checked the bearing lubrication. All fine. Suggesting deeper vibration analysis on the primary shaft.",
								highlighted: true,
							},
							{
								user: "PREDICTIVE_4_BOT",
								time: "2025-03-04",
								comment:
									"Alert generated by Neural Net Confidence 96.8%. Signature matches Testing Fatigue Cycle U-10.",
								highlighted: false,
							},
						].map((item, idx) => (
							<div
								key={idx}
								className="pb-3 border-b border-slate-700 last:border-b-0"
							>
								<div className="flex items-start gap-2 mb-1">
									{item.highlighted && (
										<div className="w-2 h-2 rounded-full bg-blue-500 mt-1.5 flex-shrink-0"></div>
									)}
									<div className="flex-1">
										<p className="text-xs font-semibold text-blue-400">
											{item.user}
										</p>
										<p className="text-xs text-slate-400">{item.time}</p>
									</div>
								</div>
								<p className="text-xs text-slate-300 ml-4">{item.comment}</p>
							</div>
						))}
					</div>

					<div className="mt-4 pt-4 border-t border-slate-700">
						<input
							type="text"
							placeholder="Add a note..."
							className="w-full bg-slate-700/50 border border-slate-600 rounded px-3 py-2 text-xs text-slate-300 placeholder-slate-500 focus:outline-none focus:border-blue-500"
						/>
					</div>
				</Card>
			</div>
		</div>
	);
}
