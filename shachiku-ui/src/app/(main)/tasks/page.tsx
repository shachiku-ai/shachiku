"use client"

import { useState, useEffect } from "react"
import { useTranslation } from "react-i18next"
import { API_URL } from "@/lib/api"
import { SiteHeader } from "@/components/site-header"
import { Loader2Icon, ListTodoIcon, CircleCheckIcon, ClockIcon, TrashIcon } from "lucide-react"
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from "@/components/ui/table"
import {
    Dialog,
    DialogContent,
    DialogHeader,
    DialogTitle,
    DialogDescription,
    DialogFooter,
} from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"
import { FileTextIcon } from "lucide-react"

type TaskLog = {
    id: number
    task_id: number
    output: string
    CreatedAt: string
}

type Task = {
    id: number
    name: string
    cron?: string
    status: string
    prompt: string
    CreatedAt: string
}

export default function TasksPage() {
    const { t } = useTranslation()
    const [tasks, setTasks] = useState<Task[]>([])
    const [loading, setLoading] = useState(true)
    const [selectedTaskLog, setSelectedTaskLog] = useState<Task | null>(null)
    const [taskLogs, setTaskLogs] = useState<TaskLog[]>([])
    const [logsLoading, setLogsLoading] = useState(false)
    const [taskToDelete, setTaskToDelete] = useState<number | null>(null)
    const [showClearConfirm, setShowClearConfirm] = useState(false)

    useEffect(() => {
        fetch(`${API_URL}/tasks`)
            .then(res => res.json())
            .then(data => {
                if (Array.isArray(data)) {
                    setTasks(data)
                }
            })
            .catch(err => console.error("Failed to fetch tasks:", err))
            .finally(() => setLoading(false))
    }, [API_URL])

    const executeClearTasks = async () => {
        setShowClearConfirm(false);
        setLoading(true);
        try {
            await fetch(`${API_URL}/tasks`, { method: "DELETE" });
            setTasks([]);
        } catch (err) {
            console.error("Failed to clear tasks:", err);
        } finally {
            setLoading(false);
        }
    };

    const handleClearTasks = () => {
        setShowClearConfirm(true);
    };

    const executeDeleteTask = async () => {
        if (taskToDelete === null) return;
        const id = taskToDelete;
        setTaskToDelete(null);
        setLoading(true);
        try {
            await fetch(`${API_URL}/tasks/${id}`, { method: "DELETE" });
            setTasks(prev => prev.filter(t => t.id !== id));
        } catch (err) {
            console.error("Failed to delete task:", err);
        } finally {
            setLoading(false);
        }
    };

    const handleDeleteTask = (id: number) => {
        setTaskToDelete(id);
    };

    const handleViewLogs = async (task: Task) => {
        setSelectedTaskLog(task);
        setLogsLoading(true);
        try {
            const res = await fetch(`${API_URL}/tasks/${task.id}/logs`);
            const data = await res.json();
            if (Array.isArray(data)) {
                setTaskLogs(data);
            } else {
                setTaskLogs([]);
            }
        } catch (err) {
            console.error("Failed to fetch task logs:", err);
            setTaskLogs([]);
        } finally {
            setLogsLoading(false);
        }
    };

    return (
        <>
            <SiteHeader title={t("tasks.title", "Executed Tasks")} />
            <div className="flex flex-1 flex-col p-4 lg:p-6 overflow-hidden">
                <h2 className="text-2xl font-bold tracking-tight">{t("tasks.title", "Agent Tasks")}</h2>
                <div className="flex items-center justify-between mt-2 mb-6">
                    <p className="text-muted-foreground">
                        {t("tasks.desc", "A log of tasks that the agent has generated and attempted to execute.")}
                    </p>
                    {tasks.length > 0 && (
                        <button
                            onClick={handleClearTasks}
                            disabled={loading}
                            className="flex auto items-center gap-2 px-3 py-1.5 text-sm bg-red-500/10 text-red-500 hover:bg-red-500/20 rounded-md transition-colors"
                        >
                            <TrashIcon className="h-4 w-4" />
                            {t("tasks.clearAll", "Clear All")}
                        </button>
                    )}
                </div>

                {loading ? (
                    <div className="flex flex-1 items-center justify-center">
                        <Loader2Icon className="h-8 w-8 animate-spin text-muted-foreground" />
                    </div>
                ) : tasks.length === 0 ? (
                    <div className="flex flex-1 items-center justify-center rounded-lg border border-dashed shadow-sm">
                        <div className="flex flex-col items-center gap-1 text-center">
                            <ListTodoIcon className="h-12 w-12 text-muted-foreground mb-4" />
                            <h3 className="text-xl font-bold tracking-tight">{t("tasks.empty", "No Tasks Yet")}</h3>
                            <p className="text-sm text-muted-foreground">
                                {t("tasks.emptyDesc", "Instruct the agent via chat to execute a specific task, and it will appear here.")}
                            </p>
                        </div>
                    </div>
                ) : (
                    <div className="rounded-md border flex-1 min-h-0 overflow-y-auto">
                        <Table>
                            <TableHeader>
                                <TableRow>
                                    <TableHead className="w-[200px] lg:w-[300px]">{t("tasks.name", "Name")}</TableHead>
                                    <TableHead>{t("tasks.status", "Status")}</TableHead>
                                    <TableHead>{t("tasks.cron", "Cron")}</TableHead>
                                    <TableHead className="whitespace-nowrap">{t("tasks.createdAt", "Created At")}</TableHead>
                                    <TableHead className="text-right w-[100px]">{t("tasks.action", "Action")}</TableHead>
                                </TableRow>
                            </TableHeader>
                            <TableBody>
                                {tasks.map((task, idx) => (
                                    <TableRow key={idx}>
                                        <TableCell className="font-medium">
                                            <div className="flex items-center gap-2">
                                                <ListTodoIcon className="h-4 w-4 text-muted-foreground" />
                                                {task.name}
                                            </div>
                                        </TableCell>
                                        <TableCell>
                                            {task.status === "completed" ? (
                                                <div className="flex items-center gap-1 text-xs px-2 py-1 rounded-full bg-green-500/10 text-green-600 w-fit">
                                                    <CircleCheckIcon className="h-3 w-3" />
                                                    {t("tasks.completed", "Completed")}
                                                </div>
                                            ) : task.status === "running" ? (
                                                <div className="flex items-center gap-1 text-xs px-2 py-1 rounded-full bg-blue-500/10 text-blue-600 w-fit">
                                                    <Loader2Icon className="h-3 w-3 animate-spin" />
                                                    {t("tasks.running", "Running")}
                                                </div>
                                            ) : task.status === "error" ? (
                                                <div className="flex items-center gap-1 text-xs px-2 py-1 rounded-full bg-red-500/10 text-red-600 w-fit">
                                                    <TrashIcon className="h-3 w-3" />
                                                    {t("tasks.error", "Error")}
                                                </div>
                                            ) : (
                                                <div className="flex items-center gap-1 text-xs px-2 py-1 rounded-full bg-amber-500/10 text-amber-600 w-fit">
                                                    <ClockIcon className="h-3 w-3" />
                                                    {t("tasks.pending", "Pending")}
                                                </div>
                                            )}
                                        </TableCell>
                                        <TableCell>
                                            {task.cron ? (
                                                <span className="text-xs text-muted-foreground font-mono bg-muted p-1 rounded inline-block">
                                                    {task.cron}
                                                </span>
                                            ) : (
                                                <span className="text-muted-foreground">-</span>
                                            )}
                                        </TableCell>
                                        <TableCell className="text-xs text-muted-foreground whitespace-nowrap">
                                            {new Date(task.CreatedAt).toLocaleString()}
                                        </TableCell>
                                        <TableCell className="text-right">
                                            <div className="flex justify-end gap-1">
                                                <button
                                                    onClick={() => handleViewLogs(task)}
                                                    disabled={loading}
                                                    className="text-primary hover:text-primary/80 hover:bg-primary/10 p-1.5 rounded-md transition-colors inline-flex"
                                                    title="View Logs"
                                                >
                                                    <FileTextIcon className="h-4 w-4" />
                                                </button>
                                                <button
                                                    onClick={() => handleDeleteTask(task.id)}
                                                    disabled={loading}
                                                    className="text-red-500 hover:text-red-700 hover:bg-red-500/10 p-1.5 rounded-md transition-colors inline-flex"
                                                    title="Delete Task"
                                                >
                                                    <TrashIcon className="h-4 w-4" />
                                                </button>
                                            </div>
                                        </TableCell>
                                    </TableRow>
                                ))}
                            </TableBody>
                        </Table>
                    </div>
                )}
            </div>

            <Dialog open={!!selectedTaskLog} onOpenChange={(open) => !open && setSelectedTaskLog(null)}>
                <DialogContent className="max-w-3xl max-h-[80vh] flex flex-col">
                    <DialogHeader>
                        <DialogTitle>{t("tasks.logsFor", "Logs for: {{name}}", { name: selectedTaskLog?.name })}</DialogTitle>
                        <DialogDescription>
                            {t("tasks.logsDesc", "Execution history and outputs for this task.")}
                        </DialogDescription>
                        {selectedTaskLog?.prompt && (
                            <div className="mt-4 p-3 bg-muted rounded-md border text-sm text-foreground/80">
                                <strong>{t("tasks.prompt", "Prompt:")}</strong> {selectedTaskLog.prompt}
                            </div>
                        )}
                    </DialogHeader>

                    <div className="flex-1 overflow-y-auto mt-4 space-y-4 pr-2">
                        {logsLoading ? (
                            <div className="flex py-12 items-center justify-center">
                                <Loader2Icon className="h-8 w-8 animate-spin text-muted-foreground" />
                            </div>
                        ) : taskLogs.length === 0 ? (
                            <div className="py-8 text-center text-muted-foreground">
                                {t("tasks.noLogs", "No execution logs found for this task.")}
                            </div>
                        ) : (
                            taskLogs.map((log) => (
                                <div key={log.id} className="rounded-md border bg-muted/50 p-4 relative">
                                    <div className="text-xs text-muted-foreground mb-2 pb-2 border-b">
                                        {new Date(log.CreatedAt).toLocaleString()}
                                    </div>
                                    <pre className="text-xs sm:text-sm whitespace-pre-wrap font-mono relative">
                                        {log.output}
                                    </pre>
                                </div>
                            ))
                        )}
                    </div>
                </DialogContent>
            </Dialog>

            <Dialog open={showClearConfirm} onOpenChange={setShowClearConfirm}>
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle>{t("tasks.confirmClearAllTitle", "Clear All Tasks")}</DialogTitle>
                        <DialogDescription className="text-base text-foreground mt-2">
                            {t("tasks.confirmClearAll", "Are you sure you want to delete all tasks?")}
                        </DialogDescription>
                    </DialogHeader>
                    <DialogFooter>
                        <Button variant="outline" onClick={() => setShowClearConfirm(false)}>{t("common.cancel", "Cancel")}</Button>
                        <Button variant="destructive" onClick={executeClearTasks}>{t("common.confirm", "Confirm")}</Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>

            <Dialog open={taskToDelete !== null} onOpenChange={(open) => !open && setTaskToDelete(null)}>
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle>{t("tasks.confirmDeleteTitle", "Delete Task")}</DialogTitle>
                        <DialogDescription className="text-base text-foreground mt-2">
                            {t("tasks.confirmDelete", "Are you sure you want to delete this task?")}
                        </DialogDescription>
                    </DialogHeader>
                    <DialogFooter>
                        <Button variant="outline" onClick={() => setTaskToDelete(null)}>{t("common.cancel", "Cancel")}</Button>
                        <Button variant="destructive" onClick={executeDeleteTask}>{t("common.confirm", "Confirm")}</Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>
        </>
    )
}
