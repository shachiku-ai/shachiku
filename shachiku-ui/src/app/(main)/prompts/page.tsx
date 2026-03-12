import { SiteHeader } from "@/components/site-header"

export default function PromptsPage() {
    return (
        <>
            <SiteHeader title="Prompt Templates" />
            <div className="flex flex-1 flex-col p-4 lg:p-6">
                <h2 className="text-2xl font-bold tracking-tight">Prompt Templates</h2>
                <p className="text-muted-foreground mt-2">
                    Manage system contexts, conversational prompts, and templated tasks for your agents.
                </p>
                <div className="mt-8 flex flex-1 items-center justify-center rounded-lg border border-dashed shadow-sm">
                    <div className="flex flex-col items-center gap-1 text-center">
                        <h3 className="text-2xl font-bold tracking-tight">No templates found</h3>
                        <p className="text-sm text-muted-foreground">
                            Create a new prompt template to standardize agent behavior.
                        </p>
                    </div>
                </div>
            </div>
        </>
    )
}
