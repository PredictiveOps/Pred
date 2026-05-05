"use client";

import { Activity, ChevronLeft, Settings } from "lucide-react";
import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";

interface AssetDetailProps {
	assetId?: string;
	onBack?: () => void;
}

export function AssetDetail({
	assetId: _assetId = "MOTOR-001",
	onBack,
}: AssetDetailProps) {
	const [telemetryData, setTelemetryData] = useState({
		temperature: 84.2,
		vibration: 4.8,
	});

	// Simulate real-time telemetry updates
	useEffect(() => {
		const interval = setInterval(() => {
			setTelemetryData({
				temperature: 80 + Math.random() * 10,
				vibration: 4.0 + Math.random() * 2,
			});
		}, 3000);

		return () => clearInterval(interval);
	}, []);

	const assetInfo = {
		name: "Induction Motor - Conveyor Line 4",
		location: "South Wing, Level 2 | Serial: PXL-402_BETA",
		status: "ACTIVE",
		healthScore: 75,
		healthStatus: "WARNING",
	};

	const mlPrediction = {
		failureProbability: 70,
		rul: 14,
		insight:
			"The increase in 2nd-order harmonic vibration suggests potential bearing misalignment. Recommend checking bearing alignment within next maintenance cycle.",
	};

	const assetSpecs = [
		{ label: "POWER RATING", value: "75 kW (100 HP)" },
		{ label: "NOMINAL RPM", value: "1785 RPM" },
		{ label: "IN-SERVICE DATE", value: "Oct 10, 2021" },
		{ label: "LAST MAINTENANCE", value: "Mar 03 04, 2024" },
		{ label: "MOUNT TYPE", value: "Rigid Foot Mount" },
	];

	return (
		<div className="space-y-6">
			{/* Header */}
			<div className="flex items-start justify-between mb-6">
				<div className="flex items-start gap-4">
					{onBack && (
						<button
							type="button"
							onClick={onBack}
							className="p-2 hover:bg-slate-800 rounded transition-colors mt-1"
						>
							<ChevronLeft className="w-5 h-5 text-slate-400" />
						</button>
					)}
					<div>
						<div className="flex items-center gap-3 mb-2">
							<h1 className="text-3xl font-bold">{assetInfo.name}</h1>
							<span className="px-3 py-1 bg-green-500/20 text-green-400 text-xs font-semibold rounded">
								{assetInfo.status}
							</span>
						</div>
						<p className="text-slate-400 text-sm">{assetInfo.location}</p>
					</div>
				</div>
				<div className="text-right">
					<div className="text-4xl font-bold text-orange-400">
						{assetInfo.healthScore}
					</div>
					<div className="text-xs text-slate-400 font-semibold">
						HEALTH SCORE
					</div>
					<div className="text-xs text-orange-400 mt-1">
						{assetInfo.healthStatus}
					</div>
				</div>
			</div>

			{/* Main Grid */}
			<div className="grid grid-cols-2 gap-6">
				{/* Left Column */}
				<div className="space-y-6">
					{/* ML Prediction Engine */}
					<Card className="bg-slate-800/50 border-slate-700 p-6">
						<h3 className="text-sm font-semibold text-slate-300 uppercase tracking-wider mb-6">
							ML Prediction Engine
						</h3>

						<div className="grid grid-cols-2 gap-6 mb-6">
							{/* Failure Probability */}
							<div>
								<div className="text-xs text-slate-400 uppercase font-semibold mb-3">
									Failure Probability
								</div>
								<div className="mb-3">
									<div className="text-3xl font-bold text-red-400">
										{mlPrediction.failureProbability}%
									</div>
								</div>
								<div className="w-full bg-slate-700 rounded-full h-2 overflow-hidden">
									<div
										className="h-full bg-red-500 rounded-full transition-all"
										style={{ width: `${mlPrediction.failureProbability}%` }}
									></div>
								</div>
							</div>

							{/* Remaining Useful Life */}
							<div>
								<div className="text-xs text-slate-400 uppercase font-semibold mb-3">
									Remaining Useful Life
								</div>
								<div className="text-3xl font-bold text-orange-400 mb-3">
									{mlPrediction.rul}
								</div>
								<div className="text-xs text-slate-400">Estimated days</div>
							</div>
						</div>

						{/* Insight */}
						<div className="bg-slate-900/50 rounded p-4 border border-slate-700">
							<div className="text-xs text-slate-400 font-semibold mb-2 uppercase">
								Analysis Insight
							</div>
							<p className="text-xs text-slate-300 leading-relaxed italic">
								"{mlPrediction.insight}"
							</p>
						</div>
					</Card>

					{/* FFT Frequency Spectrum */}
					<Card className="bg-slate-800/50 border-slate-700 p-6">
						<div className="flex justify-between items-center mb-6">
							<h3 className="text-sm font-semibold text-slate-300 uppercase tracking-wider">
								FFT Frequency Spectrum
							</h3>
							<button type="button" className="p-1 hover:bg-slate-700 rounded">
								<Settings className="w-4 h-4 text-slate-400" />
							</button>
						</div>

						<div className="h-40 flex items-end justify-around gap-2">
							<div className="flex flex-col items-center gap-2">
								<div
									className="w-12 bg-blue-500 rounded"
									style={{ height: "140px" }}
								></div>
								<span className="text-xs text-slate-400">0x Harmonics</span>
							</div>
							<div className="flex flex-col items-center gap-2">
								<div
									className="w-12 bg-orange-500 rounded"
									style={{ height: "90px" }}
								></div>
								<span className="text-xs text-slate-400">2x Harmonics</span>
							</div>
							<div className="flex flex-col items-center gap-2">
								<div
									className="w-12 bg-slate-600 rounded"
									style={{ height: "40px" }}
								></div>
								<span className="text-xs text-slate-400">3x Harmonics</span>
							</div>
						</div>

						<div className="mt-4 pt-4 border-t border-slate-700">
							<span className="text-xs text-blue-400 font-semibold">
								LIVE ANALYSIS
							</span>
						</div>
					</Card>
				</div>

				{/* Right Column */}
				<div className="space-y-6">
					{/* Telemetry Real-time Streams */}
					<Card className="bg-slate-800/50 border-slate-700 p-6">
						<div className="flex justify-between items-center mb-6">
							<h3 className="text-sm font-semibold text-slate-300 uppercase tracking-wider">
								Telemetry - Real-time Streams
							</h3>
							<div className="flex gap-2">
								{["5m", "20m", "7d"].map((range) => (
									<button
										type="button"
										key={range}
										className={`px-2 py-1 text-xs font-semibold rounded transition-colors ${
											range === "5m"
												? "bg-blue-600 text-white"
												: "bg-slate-700 text-slate-400 hover:bg-slate-600"
										}`}
									>
										{range}
									</button>
								))}
							</div>
						</div>

						{/* Temperature */}
						<div className="mb-6">
							<div className="flex justify-between items-center mb-2">
								<span className="text-xs text-slate-400 uppercase font-semibold">
									Temperature (°C)
								</span>
								<span className="text-sm font-semibold text-orange-400">
									CURRENT: {telemetryData.temperature.toFixed(1)}°C
								</span>
							</div>
							<div className="h-20 bg-slate-900 rounded flex items-center justify-center">
								<svg
									className="w-full h-full p-2"
									viewBox="0 0 100 20"
									preserveAspectRatio="none"
								>
									<polyline
										points="0,12 10,10 20,8 30,6 40,8 50,5 60,7 70,4 80,6 90,3 100,5"
										fill="none"
										stroke="#f59e0b"
										strokeWidth="1"
									/>
								</svg>
							</div>
						</div>

						{/* Vibration */}
						<div>
							<div className="flex justify-between items-center mb-2">
								<span className="text-xs text-slate-400 uppercase font-semibold">
									Vibration (mm/s)
								</span>
								<span className="text-sm font-semibold text-red-400">
									CURRENT: {telemetryData.vibration.toFixed(1)} mm/s
								</span>
							</div>
							<div className="h-20 bg-slate-900 rounded flex items-center justify-center">
								<svg
									className="w-full h-full p-2"
									viewBox="0 0 100 20"
									preserveAspectRatio="none"
								>
									<polyline
										points="0,10 10,8 20,12 30,7 40,11 50,9 60,13 70,6 80,10 90,8 100,11"
										fill="none"
										stroke="#ef4444"
										strokeWidth="1"
									/>
								</svg>
							</div>
						</div>
					</Card>

					{/* Asset Specifications */}
					<Card className="bg-slate-800/50 border-slate-700 p-6">
						<div className="flex justify-between items-center mb-6">
							<h3 className="text-sm font-semibold text-slate-300 uppercase tracking-wider">
								Asset Specifications
							</h3>
							<button type="button" className="p-1 hover:bg-slate-700 rounded">
								<Activity className="w-4 h-4 text-slate-400" />
							</button>
						</div>

						<div className="space-y-4">
							{assetSpecs.map((spec, idx) => (
								<div
									key={idx}
									className="flex justify-between items-start pb-4 border-b border-slate-700 last:border-b-0"
								>
									<span className="text-xs text-slate-400 font-semibold uppercase">
										{spec.label}
									</span>
									<span className="text-sm font-semibold text-slate-200">
										{spec.value}
									</span>
								</div>
							))}
						</div>

						{/* Sensor Node Status */}
						<div className="text-xs text-slate-400 font-semibold uppercase">
							Operating Environment
						</div>
						<div className="mt-2 pt-2 border-t border-slate-700">
							<div className="text-xs text-slate-400 font-semibold uppercase mb-4">
								Sensor Node Status
							</div>
							<div className="space-y-3">
								<div className="flex items-center justify-between p-3 bg-slate-900/50 rounded border border-slate-700">
									<div className="flex items-center gap-2">
										<div className="w-2 h-2 bg-green-500 rounded-full"></div>
										<span className="text-xs text-slate-300">
											Vibration Sensor (ACCEL-01)
										</span>
									</div>
									<span className="text-xs font-semibold text-green-400">
										CONNECTED
									</span>
								</div>
								<div className="flex items-center justify-between p-3 bg-slate-900/50 rounded border border-slate-700">
									<div className="flex items-center gap-2">
										<div className="w-2 h-2 bg-green-500 rounded-full"></div>
										<span className="text-xs text-slate-300">
											Temperature Sensor (TEMP-01)
										</span>
									</div>
									<span className="text-xs font-semibold text-green-400">
										CONNECTED
									</span>
								</div>
								<div className="flex items-center justify-between p-3 bg-slate-900/50 rounded border border-slate-700">
									<div className="flex items-center gap-2">
										<div className="w-2 h-2 bg-green-500 rounded-full"></div>
										<span className="text-xs text-slate-300">
											Power Monitor (PWR-01)
										</span>
									</div>
									<span className="text-xs font-semibold text-green-400">
										CONNECTED
									</span>
								</div>
							</div>
						</div>
					</Card>
				</div>
			</div>

			{/* Schedule Inspection Button */}
			<div className="flex justify-end">
				<Button className="bg-blue-600 hover:bg-blue-700 px-8 font-semibold uppercase text-sm">
					SCHEDULE MANUAL INSPECTION
				</Button>
			</div>
		</div>
	);
}
