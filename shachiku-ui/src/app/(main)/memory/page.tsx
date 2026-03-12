"use client"

import { useState, useEffect } from "react"
import { useTranslation } from "react-i18next"
import { API_URL } from "@/lib/api"
import { SiteHeader } from "@/components/site-header"
import { BrainIcon, DatabaseIcon, Trash2Icon, Loader2Icon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"

type Fact = {
    id: string
    content: string
    timestamp: string
}


export default function MemoryLogsPage() {
    const { t } = useTranslation()
    const [facts, setFacts] = useState<Fact[]>([])
    const [loadingFacts, setLoadingFacts] = useState(true)

    const fetchData = async () => {
        setLoadingFacts(true)
        try {
            const factsRes = await fetch(`${API_URL}/memory/long`)
            const factsData = await factsRes.json()

            if (Array.isArray(factsData)) {
                setFacts(factsData.sort((a, b) => parseInt(b.timestamp) - parseInt(a.timestamp)))
            } else {
                setFacts([])
            }
        } catch (err) {
            console.error("Failed to load memory:", err)
        } finally {
            setLoadingFacts(false)
        }
    }

    useEffect(() => {
        fetchData()
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [])

    const handleDelete = async (id: string) => {
        try {
            await fetch(`${API_URL}/memory/long/${id}`, { method: "DELETE" })
            setFacts((prev) => prev.filter((fact) => fact.id !== id))
        } catch (err) {
            console.error("Failed to delete memory:", err)
        }
    }


    return (
        <>
            <SiteHeader title={t("memory.title", "Memory Logs")} />
            <div className="flex flex-1 flex-col p-4 lg:p-6 space-y-6 overflow-y-auto w-full max-w-5xl mx-auto">
                <div className="flex justify-between items-center">
                    <div>
                        <h2 className="text-2xl font-bold tracking-tight">{t("memory.title", "Memory Logs")}</h2>
                        <p className="text-muted-foreground mt-2">
                            {t("memory.desc", "Monitor your agent's memory storage, semantic search indexing, and context retrievals.")}
                        </p>
                    </div>
                    <div className="flex gap-2">
                        <Button variant="outline" onClick={fetchData} disabled={loadingFacts}>
                            <DatabaseIcon className="mr-2 h-4 w-4" />
                            {t("memory.refresh", "Refresh")}
                        </Button>
                    </div>
                </div>

                <div className="grid gap-4 mt-8">
                    <Card>
                        <CardHeader className="flex flex-row items-center justify-between pb-2">
                            <div className="space-y-1">
                                <CardTitle className="text-base font-semibold flex items-center">
                                    <BrainIcon className="mr-2 h-4 w-4 text-primary" />
                                    {t("memory.longTerm", "Long-Term Memory (Vector Knowledge Base)")}
                                </CardTitle>
                                <CardDescription>{t("memory.longTermDesc", "Semantic facts automatically extracted and stored by the agent")}</CardDescription>
                            </div>
                        </CardHeader>
                        <CardContent>
                            {loadingFacts ? (
                                <div className="flex py-10 items-center justify-center">
                                    <Loader2Icon className="h-6 w-6 animate-spin text-muted-foreground" />
                                </div>
                            ) : facts.length === 0 ? (
                                <div className="flex flex-col py-10 items-center justify-center text-center">
                                    <DatabaseIcon className="h-10 w-10 text-muted-foreground/50 mb-4" />
                                    <p className="text-sm font-medium text-muted-foreground">{t("memory.idle", "Memory system is idle")}</p>
                                    <p className="text-xs text-muted-foreground mt-1">{t("memory.empty", "No long-term memories have been stored yet.")}</p>
                                </div>
                            ) : (
                                <div className="space-y-3 overflow-y-auto pr-2">
                                    {facts.map((fact) => (
                                        <div key={fact.id} className="flex items-start justify-between space-x-4 border rounded-lg p-4 transition-colors hover:bg-muted/50">
                                            <div className="space-y-1 pr-4">
                                                <p className="text-sm font-medium leading-relaxed">{fact.content}</p>
                                                <p className="text-xs text-muted-foreground mt-2">
                                                    {new Date(parseInt(fact.timestamp) * 1000).toLocaleString()} • ID: {fact.id.split("-")[0]}
                                                </p>
                                            </div>
                                            <Button variant="ghost" size="icon" onClick={() => handleDelete(fact.id)} className="shrink-0 h-8 w-8 text-destructive opacity-80 hover:bg-destructive/10 hover:text-destructive hover:opacity-100">
                                                <Trash2Icon className="h-4 w-4" />
                                                <span className="sr-only">Delete</span>
                                            </Button>
                                        </div>
                                    ))}
                                </div>
                            )}
                        </CardContent>
                    </Card>


                </div>
            </div>
        </>
    )
}
