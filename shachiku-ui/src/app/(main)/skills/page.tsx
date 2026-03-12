"use client"

import { useState, useEffect } from "react"
import { useTranslation } from "react-i18next"
import { API_URL } from "@/lib/api"
import { SiteHeader } from "@/components/site-header"
import { Loader2Icon, WrenchIcon, TrashIcon } from "lucide-react"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from "@/components/ui/table"

type Skill = {
    name: string
    description: string
    is_builtin: boolean
}

export default function SkillsPage() {
    const { t } = useTranslation()
    const [skills, setSkills] = useState<Skill[]>([])
    const [loading, setLoading] = useState(true)
    const [skillToDelete, setSkillToDelete] = useState<string | null>(null)

    useEffect(() => {
        fetch(`${API_URL}/skills`)
            .then(res => res.json())
            .then(data => {
                if (Array.isArray(data)) {
                    setSkills(data)
                }
            })
            .catch(err => console.error("Failed to fetch skills:", err))
            .finally(() => setLoading(false))
    }, [API_URL])

    const executeDeleteSkill = async () => {
        if (!skillToDelete) return;
        const name = skillToDelete;
        setSkillToDelete(null);
        setLoading(true);
        try {
            await fetch(`${API_URL}/skills/${encodeURIComponent(name)}`, { method: "DELETE" });
            setSkills(prev => prev.filter(s => s.name !== name));
        } catch (err) {
            console.error("Failed to delete skill:", err);
        } finally {
            setLoading(false);
        }
    };

    const handleDeleteSkill = (name: string) => {
        setSkillToDelete(name)
    };

    return (
        <>
            <SiteHeader title={t("skills.title", "Agent Skills")} />
            <div className="flex flex-1 flex-col p-4 lg:p-6 overflow-hidden">
                <h2 className="text-2xl font-bold tracking-tight">{t("skills.title", "Agent Skills")}</h2>
                <p className="text-muted-foreground mt-2 mb-6">
                    {t("skills.desc", "Functions and dynamic skills the agent can execute during reasoning loops. Ask the agent via chat to create new skills.")}
                </p>

                {loading ? (
                    <div className="flex flex-1 items-center justify-center">
                        <Loader2Icon className="h-8 w-8 animate-spin text-muted-foreground" />
                    </div>
                ) : skills.length === 0 ? (
                    <div className="flex flex-1 items-center justify-center rounded-lg border border-dashed shadow-sm">
                        <div className="flex flex-col items-center gap-1 text-center">
                            <WrenchIcon className="h-12 w-12 text-muted-foreground mb-4" />
                            <h3 className="text-xl font-bold tracking-tight">{t("skills.empty", "System Skills Empty")}</h3>
                            <p className="text-sm text-muted-foreground">
                                {t("skills.emptyDesc", "Define the first set of functional schema tools before executing your core intelligence pipeline.")}
                            </p>
                        </div>
                    </div>
                ) : (
                    <div className="rounded-md border flex-1 min-h-0 overflow-y-auto">
                        <Table>
                            <TableHeader>
                                <TableRow>
                                    <TableHead className="w-[250px]">{t("skills.name", "Name")}</TableHead>
                                    <TableHead>{t("skills.description", "Description")}</TableHead>
                                    <TableHead className="text-right w-[100px]">{t("skills.action", "Action")}</TableHead>
                                </TableRow>
                            </TableHeader>
                            <TableBody>
                                {skills.map((skill, idx) => (
                                    <TableRow key={idx}>
                                        <TableCell className="font-medium">
                                            <div className="flex items-center gap-2">
                                                <WrenchIcon className="h-4 w-4 text-muted-foreground" />
                                                {skill.name}
                                            </div>
                                        </TableCell>
                                        <TableCell className="whitespace-pre-wrap text-sm text-muted-foreground">{skill.description}</TableCell>
                                        <TableCell className="text-right">
                                            {!skill.is_builtin && (
                                                <button
                                                    onClick={() => handleDeleteSkill(skill.name)}
                                                    disabled={loading}
                                                    className="text-red-500 hover:text-red-700 hover:bg-red-500/10 p-1.5 rounded-md transition-colors inline-flex"
                                                    title="Delete Skill"
                                                >
                                                    <TrashIcon className="h-4 w-4" />
                                                </button>
                                            )}
                                        </TableCell>
                                    </TableRow>
                                ))}
                            </TableBody>
                        </Table>
                    </div>
                )}
            </div>

            <Dialog open={!!skillToDelete} onOpenChange={(open) => !open && setSkillToDelete(null)}>
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle>{t("skills.confirmDeleteTitle", "Delete Skill")}</DialogTitle>
                        <DialogDescription className="text-base text-foreground mt-2">
                            {skillToDelete && t("skills.confirmDelete", "Are you sure you want to delete the skill '{{name}}'?", { name: skillToDelete })}
                        </DialogDescription>
                    </DialogHeader>
                    <DialogFooter>
                        <Button variant="outline" onClick={() => setSkillToDelete(null)}>{t("common.cancel", "Cancel")}</Button>
                        <Button variant="destructive" onClick={executeDeleteSkill}>{t("common.confirm", "Confirm")}</Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>
        </>
    )
}
