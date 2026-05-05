"use client";

import { AlertCircle, AlertTriangle, CheckCircle } from "lucide-react";

export default function DashboardPage() {
	return (
		<>
			<div className="flex justify-between items-start mb-6">
				<div>
					<h1 className="text-3xl font-bold">Control Room Dashboard</h1>
					<p className="text-slate-400 text-sm mt-1">
						Monitoring 124 active industrial motor units
					</p>
				</div>
				<div className="flex gap-3">
					<button
						type="button"
						className="px-4 py-2 border border-slate-600 rounded hover:bg-slate-800 text-sm font-medium flex items-center gap-2"
					>
						⚙️ FILTER
					</button>
					<button
						type="button"
						className="px-4 py-2 bg-blue-600 rounded hover:bg-blue-700 text-sm font-medium flex items-center gap-2"
					>
						↓ EXPORT REPORT
					</button>
				</div>
			</div>

			<div className="grid grid-cols-4 gap-4 mb-6">
				{/* Status Cards */}
				<div className="bg-slate-800 border border-slate-700 rounded p-4">
					<div className="text-slate-400 text-xs font-semibold mb-3">
						NORMAL OPERATION
					</div>
					<div className="text-3xl font-bold">112</div>
					<div className="text-slate-500 text-xs mt-1">UNITS</div>
					<div className="mt-3 h-1 bg-gradient-to-r from-green-500 to-emerald-600 rounded"></div>
				</div>

				<div className="bg-slate-800 border border-slate-700 rounded p-4">
					<div className="text-slate-400 text-xs font-semibold mb-3">
						WARNING STATUS
					</div>
					<div className="text-3xl font-bold text-yellow-500">9</div>
					<div className="text-slate-500 text-xs mt-1">UNITS</div>
					<div className="mt-3 h-1 bg-gradient-to-r from-yellow-500 to-orange-600 rounded"></div>
				</div>

				<div className="bg-slate-800 border border-slate-700 rounded p-4">
					<div className="text-slate-400 text-xs font-semibold mb-3">
						CRITICAL ALERTS
					</div>
					<div className="text-3xl font-bold text-red-500">3</div>
					<div className="text-slate-500 text-xs mt-1">UNITS</div>
					<div className="mt-3 h-1 bg-gradient-to-r from-red-500 to-rose-600 rounded"></div>
				</div>

				<div className="bg-slate-800 border border-slate-700 rounded p-4">
					<div className="text-slate-400 text-xs font-semibold mb-3">
						ACTIVE ALERTS
					</div>
					<div className="text-3xl font-bold">
						<span className="text-red-500">1</span>
						<span className="text-xs ml-2 bg-red-600 px-2 py-1 rounded">
							NEW
						</span>
					</div>
					<div className="text-slate-500 text-xs mt-1">UNITS</div>
				</div>
			</div>

			<div className="grid grid-cols-3 gap-4 mb-6">
				{/* Motor Unit Cards */}
				<div className="bg-slate-800 border border-slate-700 rounded p-4">
					<div className="flex items-center justify-between mb-4">
						<div className="text-sm font-semibold">⚙️ MOTOR UNIT A-102</div>
						<span className="text-xs bg-red-600 px-2 py-1 rounded">
							CRITICAL
						</span>
					</div>
					<div className="grid grid-cols-2 gap-4 mb-4">
						<div>
							<div className="text-xs text-slate-400">VIBRATION</div>
							<div className="text-2xl font-bold">
								4.82<span className="text-xs text-slate-400">mm/s</span>
							</div>
							<div className="mt-1 h-0.5 bg-red-500 rounded"></div>
						</div>
						<div>
							<div className="text-xs text-slate-400">TEMPERATURE</div>
							<div className="text-2xl font-bold">
								82.4<span className="text-xs text-slate-400">°C</span>
							</div>
							<div className="mt-1 h-0.5 bg-yellow-500 rounded"></div>
						</div>
					</div>
					<div className="bg-slate-900 rounded p-3">
						<div className="text-xs text-slate-500 mb-1">
							PREDICTIVE RUL (REMAINING USEFUL LIFE)
						</div>
						<div className="text-lg font-bold text-orange-400">14 DAYS</div>
						<div className="text-xs text-red-400 mt-1">ACTION REQUIRED</div>
					</div>
				</div>

				<div className="bg-slate-800 border border-slate-700 rounded p-4">
					<div className="flex items-center justify-between mb-4">
						<div className="text-sm font-semibold">⚙️ MOTOR UNIT B-044</div>
						<span className="text-xs bg-yellow-600 px-2 py-1 rounded">
							WARNING
						</span>
					</div>
					<div className="grid grid-cols-2 gap-4 mb-4">
						<div>
							<div className="text-xs text-slate-400">VIBRATION</div>
							<div className="text-2xl font-bold">
								2.11<span className="text-xs text-slate-400">mm/s</span>
							</div>
							<div className="mt-1 h-0.5 bg-green-500 rounded"></div>
						</div>
						<div>
							<div className="text-xs text-slate-400">TEMPERATURE</div>
							<div className="text-2xl font-bold">
								74.2<span className="text-xs text-slate-400">°C</span>
							</div>
							<div className="mt-1 h-0.5 bg-orange-500 rounded"></div>
						</div>
					</div>
					<div className="bg-slate-900 rounded p-3">
						<div className="text-xs text-slate-500 mb-1">PREDICTIVE RUL</div>
						<div className="text-lg font-bold text-green-400">124 DAYS</div>
						<div className="text-xs text-slate-400 mt-1">MAINTAIN PLAN</div>
					</div>
				</div>

				{/* Right Sidebar - Alerts */}
				<div className="space-y-3">
					<div className="bg-slate-800 border border-slate-700 rounded p-3">
						<div className="flex gap-2 items-start">
							<AlertCircle className="w-4 h-4 text-red-500 mt-1 flex-shrink-0" />
							<div className="text-xs">
								<div className="font-semibold text-red-500">CRITICAL</div>
								<div className="text-slate-300 mt-1">
									Bearing overheating detected in A-102
								</div>
								<div className="text-slate-500 text-xs mt-1">
									Temp: TH-68 reached threshold 85°C
								</div>
							</div>
						</div>
					</div>

					<div className="bg-slate-800 border border-slate-700 rounded p-3">
						<div className="flex gap-2 items-start">
							<AlertTriangle className="w-4 h-4 text-yellow-500 mt-1 flex-shrink-0" />
							<div className="text-xs">
								<div className="font-semibold text-yellow-500">WARNING</div>
								<div className="text-slate-300 mt-1">
									Vibration deviation B-044
								</div>
								<div className="text-slate-500 text-xs mt-1">
									Harmonic resonance detected Phase 2
								</div>
							</div>
						</div>
					</div>

					<div className="bg-slate-800 border border-slate-700 rounded p-3">
						<div className="flex gap-2 items-start">
							<CheckCircle className="w-4 h-4 text-blue-500 mt-1 flex-shrink-0" />
							<div className="text-xs">
								<div className="font-semibold text-blue-500">INFO</div>
								<div className="text-slate-300 mt-1">
									Routine scheduled for C-Series
								</div>
								<div className="text-slate-500 text-xs mt-1">
									PM cycle starts in 24 hours
								</div>
							</div>
						</div>
					</div>

					<div className="bg-slate-800 border border-slate-700 rounded p-3">
						<div className="flex gap-2 items-start">
							<AlertCircle className="w-4 h-4 text-red-500 mt-1 flex-shrink-0" />
							<div className="text-xs">
								<div className="font-semibold text-red-500">CRITICAL</div>
								<div className="text-slate-300 mt-1">
									Connection loss: Sensor Node 12
								</div>
								<div className="text-slate-500 text-xs mt-1">
									Network packet drop &gt; 98%
								</div>
							</div>
						</div>
					</div>

					<button
						type="button"
						className="w-full mt-2 text-xs text-slate-400 hover:text-slate-300 py-2"
					>
						VIEW ALL EVENTS
					</button>
				</div>
			</div>

			{/* Telemetry Chart */}
			<div className="bg-slate-800 border border-slate-700 rounded p-4">
				<div className="text-sm font-semibold mb-4">
					SYSTEM-WIDE TELEMETRY (24H)
				</div>
				<div className="flex items-center gap-2 mb-4">
					<div className="w-2 h-2 rounded-full bg-green-500"></div>
					<span className="text-xs text-slate-400">LOAD</span>
					<div className="w-2 h-2 rounded-full bg-orange-500 ml-4"></div>
					<span className="text-xs text-slate-400">VIBRATION</span>
				</div>
				<div className="h-32 bg-slate-900 rounded flex items-center justify-center text-slate-500">
					<svg
						className="w-full h-full p-4"
						viewBox="0 0 400 100"
						preserveAspectRatio="none"
						role="img"
						aria-labelledby="system-telemetry-chart-title"
					>
						<title id="system-telemetry-chart-title">
							System-wide telemetry chart showing load and vibration over 24
							hours
						</title>
						<polyline
							points="0,60 50,50 100,55 150,40 200,45 250,35 300,50 350,30 400,45"
							fill="none"
							stroke="#10b981"
							strokeWidth="2"
							opacity="0.6"
						/>
						<polyline
							points="0,70 50,65 100,72 150,60 200,68 250,55 300,75 350,50 400,65"
							fill="none"
							stroke="#f59e0b"
							strokeWidth="2"
							opacity="0.6"
						/>
					</svg>
				</div>
			</div>
		</>
	);
}
