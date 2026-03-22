"use client"

import { useState, useEffect } from "react"
import { useTranslation } from "react-i18next"
import { API_URL } from "@/lib/api"
import { SiteHeader } from "@/components/site-header"
import { Loader2Icon, ActivityIcon, CpuIcon, HashIcon } from "lucide-react"
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from "@/components/ui/table"
import { Bar, BarChart, CartesianGrid, XAxis, YAxis } from "recharts"
import {
    ChartConfig,
    ChartContainer,
    ChartTooltip,
    ChartTooltipContent,
    ChartLegend,
    ChartLegendContent,
} from "@/components/ui/chart"



type DailyTokenUsage = {
    date: string
    input_tokens: number
    output_tokens: number
}

type TaskTokenUsage = {
    task_id: number
    task_name: string
    input_tokens: number
    output_tokens: number
}

type DashboardMetrics = {
    daily_usage: DailyTokenUsage[]
    task_usage: TaskTokenUsage[]
}

export default function TokenDashboardPage() {
    const { t } = useTranslation()
    const [metrics, setMetrics] = useState<DashboardMetrics | null>(null)
    const [loading, setLoading] = useState(true)

    const chartConfig = {
        input_tokens: {
            label: t("dashboard.inputTokensShort", "Input Tokens"),
            color: "#96C3FD",
        },
        output_tokens: {
            label: t("dashboard.outputTokensShort", "Output Tokens"),
            color: "#4D7CFF",
        },
    } satisfies ChartConfig

    useEffect(() => {
        fetch(`${API_URL}/tokens/dashboard`)
            .then(res => res.json())
            .then(data => {
                if (data && data.daily_usage) {
                    setMetrics(data)
                }
            })
            .catch(err => console.error("Failed to fetch token metrics:", err))
            .finally(() => setLoading(false))
    }, [API_URL])

    const totalInput = metrics?.daily_usage.reduce((sum, day) => sum + day.input_tokens, 0) || 0
    const totalOutput = metrics?.daily_usage.reduce((sum, day) => sum + day.output_tokens, 0) || 0
    const totalTokens = totalInput + totalOutput

    return (
        <>
            <SiteHeader title={t("dashboard.title", "Token Usage Dashboard")} />
            <div className="flex flex-1 flex-col p-4 lg:p-6 overflow-hidden overflow-y-auto">
                <div className="flex items-center space-x-2 mb-6">
                    <ActivityIcon className="h-6 w-6 text-primary" />
                    <h2 className="text-2xl font-bold tracking-tight">{t("dashboard.overview", "Token Usage Overview")}</h2>
                </div>

                {loading ? (
                    <div className="flex flex-1 items-center justify-center">
                        <Loader2Icon className="h-8 w-8 animate-spin text-muted-foreground" />
                    </div>
                ) : !metrics ? (
                    <div className="flex flex-1 items-center justify-center rounded-lg border border-dashed shadow-sm p-12">
                        <div className="flex flex-col items-center gap-1 text-center">
                            <ActivityIcon className="h-12 w-12 text-muted-foreground mb-4" />
                            <h3 className="text-xl font-bold tracking-tight">{t("dashboard.noData", "No Data Available")}</h3>
                            <p className="text-sm text-muted-foreground">
                                {t("dashboard.noDataDesc", "Token usage data could not be loaded. Please ensure the agent has executed tasks.")}
                            </p>
                        </div>
                    </div>
                ) : (
                    <div className="flex flex-col gap-6">
                        {/* Summary Cards */}
                        <div className="grid gap-4 md:grid-cols-3">
                            <div className="rounded-xl border bg-card text-card-foreground shadow">
                                <div className="p-6 flex flex-row items-center justify-between space-y-0 pb-2">
                                    <h3 className="tracking-tight text-sm font-medium">{t("dashboard.totalTokens", "Total Tokens (30d)")}</h3>
                                    <HashIcon className="h-4 w-4 text-muted-foreground" />
                                </div>
                                <div className="p-6 pt-0">
                                    <div className="text-2xl font-bold">{totalTokens.toLocaleString()}</div>
                                </div>
                            </div>
                            <div className="rounded-xl border bg-card text-card-foreground shadow">
                                <div className="p-6 flex flex-row items-center justify-between space-y-0 pb-2">
                                    <h3 className="tracking-tight text-sm font-medium">{t("dashboard.inputTokens", "Input Tokens (30d)")}</h3>
                                    <CpuIcon className="h-4 w-4 text-muted-foreground" />
                                </div>
                                <div className="p-6 pt-0">
                                    <div className="text-2xl font-bold">{totalInput.toLocaleString()}</div>
                                </div>
                            </div>
                            <div className="rounded-xl border bg-card text-card-foreground shadow">
                                <div className="p-6 flex flex-row items-center justify-between space-y-0 pb-2">
                                    <h3 className="tracking-tight text-sm font-medium">{t("dashboard.outputTokens", "Output Tokens (30d)")}</h3>
                                    <ActivityIcon className="h-4 w-4 text-muted-foreground" />
                                </div>
                                <div className="p-6 pt-0">
                                    <div className="text-2xl font-bold">{totalOutput.toLocaleString()}</div>
                                </div>
                            </div>
                        </div>

                        {/* Chart */}
                        <div className="rounded-xl border bg-card text-card-foreground shadow flex flex-col pt-6">
                            <h3 className="font-semibold leading-none tracking-tight px-6 mb-4">{t("dashboard.chartTitle", "Daily Token Usage (Last 30 Days)")}</h3>
                            <div className="h-[350px] w-full px-6 pb-6 pt-2">
                                <ChartContainer config={chartConfig} className="h-full w-full">
                                    <BarChart accessibilityLayer data={metrics.daily_usage} margin={{ top: 10, right: 10, left: 0, bottom: 0 }}>
                                        <CartesianGrid vertical={false} strokeDasharray="3 3" />
                                        <XAxis
                                            dataKey="date"
                                            tickLine={false}
                                            tickMargin={10}
                                            axisLine={false}
                                        />
                                        <YAxis
                                            tickLine={false}
                                            axisLine={false}
                                            tickMargin={10}
                                            tickFormatter={(val) => val >= 1000 ? (val / 1000).toFixed(1) + 'k' : val}
                                        />
                                        <ChartTooltip
                                            cursor={false}
                                            content={<ChartTooltipContent />}
                                        />
                                        <ChartLegend content={<ChartLegendContent />} />
                                        <Bar dataKey="input_tokens" fill="var(--color-input_tokens)" radius={4} />
                                        <Bar dataKey="output_tokens" fill="var(--color-output_tokens)" radius={4} />
                                    </BarChart>
                                </ChartContainer>
                            </div>
                        </div>

                        {/* Task Table */}
                        <div className="rounded-xl border bg-card text-card-foreground shadow flex flex-col overflow-hidden">
                            <div className="p-6 pb-4">
                                <h3 className="font-semibold leading-none tracking-tight">{t("dashboard.tableTitle", "Token Usage by Task")}</h3>
                                <p className="text-sm text-muted-foreground mt-1.5">{t("dashboard.tableDesc", "A breakdown of the total tokens consumed per task.")}</p>
                            </div>
                            <div className="px-6 pb-6 w-full overflow-auto">
                                <div className="rounded-md border h-[300px] overflow-y-auto">
                                    <Table>
                                        <TableHeader className="bg-muted/50 sticky top-0 z-10">
                                            <TableRow>
                                                <TableHead>{t("dashboard.taskName", "Task Name")}</TableHead>
                                                <TableHead className="text-right">{t("dashboard.tableInput", "Input Tokens")}</TableHead>
                                                <TableHead className="text-right">{t("dashboard.tableOutput", "Output Tokens")}</TableHead>
                                                <TableHead className="text-right">{t("dashboard.tableTotal", "Total Tokens")}</TableHead>
                                            </TableRow>
                                        </TableHeader>
                                        <TableBody>
                                            {metrics.task_usage.length === 0 ? (
                                                <TableRow>
                                                    <TableCell colSpan={4} className="h-24 text-center text-muted-foreground">
                                                        {t("dashboard.noTaskData", "No task data available.")}
                                                    </TableCell>
                                                </TableRow>
                                            ) : (
                                                metrics.task_usage.map((task, idx) => {
                                                    const total = task.input_tokens + task.output_tokens;
                                                    return (
                                                        <TableRow key={idx}>
                                                            <TableCell className="font-medium">
                                                                {task.task_id === 0 ? t("dashboard.systemTask", "System / Chat Interface") : (task.task_name || `${t("dashboard.unknownTask", "Unknown Task #")}${task.task_id}`)}
                                                            </TableCell>
                                                            <TableCell className="text-right">{task.input_tokens.toLocaleString()}</TableCell>
                                                            <TableCell className="text-right">{task.output_tokens.toLocaleString()}</TableCell>
                                                            <TableCell className="text-right font-bold text-primary">{total.toLocaleString()}</TableCell>
                                                        </TableRow>
                                                    )
                                                })
                                            )}
                                        </TableBody>
                                    </Table>
                                </div>
                            </div>
                        </div>

                    </div>
                )}
            </div>
        </>
    )
}
