"use client"
/* eslint-disable @typescript-eslint/no-unused-vars */

import { useState, useEffect, useRef } from "react"
import { useTranslation } from "react-i18next"
import { SendIcon, BotIcon, UserIcon, Loader2Icon, ChevronRightIcon, BrushCleaning, FileIcon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { SiteHeader } from "@/components/site-header"
import ReactMarkdown from "react-markdown"
import remarkGfm from "remark-gfm"
import rehypeRaw from "rehype-raw"
import { useChat } from "@/providers/chat-provider"

export default function ChatPage() {
  const { t } = useTranslation()
  const {
    messages, input, setInput,
    loading, fetching, abortController, setAbortController,
    handleSend: handleSendContext, executeClearShortMemory
  } = useChat()
  const [showClearConfirm, setShowClearConfirm] = useState(false)
  const messagesEndRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" })
  }, [messages])

  const handleClearShortMemory = () => {
    setShowClearConfirm(true)
  }

  const handleConfirmClear = async () => {
    setShowClearConfirm(false)
    await executeClearShortMemory()
  }

  const handleSend = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!input.trim() || loading) return

    const msg = input.trim();
    setInput("")

    await handleSendContext(e, msg)
  }

  const renderMessageText = (content: string, isUser: boolean) => {
    if (!content) return null;
    return (
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        rehypePlugins={[rehypeRaw]}
        components={{
          ...(isUser ? {
            p: ({ node: _, ...props }) => <p className="mb-2 last:mb-0" {...props} />,
            a: ({ node: _, ...props }) => <a className="underline hover:opacity-80" {...props} />
          } : {}),
          img: ({ node: _, ...props }) => (
            // eslint-disable-next-line @next/next/no-img-element
            <img className="max-w-full h-auto rounded-lg my-2 shadow-sm" {...props} alt={props.alt || ""} />
          )
        }}
      >
        {content}
      </ReactMarkdown>
    );
  };

  const extractMessageParts = (content: string) => {
    if (!content) return { files: [], text: "" };
    const parts = content.split(/(^@\/.*$)/m);
    const files: string[] = [];
    const textParts: string[] = [];
    parts.forEach(p => {
      if (p.startsWith("@/")) {
        files.push(p.substring(1).trim());
      } else if (p) {
        textParts.push(p);
      }
    });
    return { files, text: textParts.join("").trim() };
  };

  return (
    <div className="absolute inset-0 flex flex-col overflow-hidden md:rounded-xl">
      <SiteHeader title={t("navMain.chat", "Chat & History")}>
        <Button variant="outline" size="sm" onClick={handleClearShortMemory}>
          <BrushCleaning className="h-4 w-4" />
          <span className="hidden md:block ml-2">
            {t("chat.clearShortMemory", "Clear Short Memory")}
          </span>
        </Button>
      </SiteHeader>
      <div className="flex flex-1 flex-col overflow-hidden">
        <div className="flex-1 overflow-y-auto p-4 lg:p-6 space-y-4 min-h-0">
          {fetching ? (
            <div className="flex items-center justify-center h-full">
              <Loader2Icon className="h-8 w-8 animate-spin text-muted-foreground" />
            </div>
          ) : messages.length === 0 ? (
            <div className="flex h-full flex-col items-center justify-center text-center">
              <BotIcon className="h-12 w-12 text-muted-foreground mb-4" />
              <h3 className="text-2xl font-bold tracking-tight">{t("chat.title", "Agent Conversation")}</h3>
              <p className="text-sm text-muted-foreground mt-2 max-w-sm">
                {t("chat.empty", "History is currently empty. Start typing to send your first message to the agent.")}
              </p>
            </div>
          ) : (
            messages.map((msg, idx) => {
              const { files, text } = extractMessageParts(msg.Content || "");
              const hasTextBubble = text || msg.Thought || (loading && idx === messages.length - 1);

              return (
                <div
                  key={idx}
                  className={`flex gap-3 ${msg.Role === "user" ? "justify-end" : "justify-start"}`}
                >
                  {msg.Role !== "user" && (
                    <div className="flex h-8 w-8 shrink-0 select-none items-center justify-center rounded-md border bg-background shadow-sm">
                      {msg.Role === "agent" ? <BotIcon className="h-4 w-4" /> : <BotIcon className="h-4 w-4 text-destructive" />}
                    </div>
                  )}

                  <div className={`flex flex-col gap-2 max-w-[80%] ${msg.Role === "user" ? "items-end" : "items-start"}`}>
                    {files.length > 0 && (
                      <div className="flex flex-col gap-2 w-full">
                        {files.map((path, i) => {
                          const filename = path.split("/").pop() || path;
                          return (
                            <div key={i} className={`p-3 bg-background rounded-lg border flex items-center gap-3 shadow-sm text-foreground w-max max-w-full ${msg.Role === "user" ? "self-end" : "self-start"}`}>
                              <FileIcon className="h-8 w-8 text-primary shrink-0" />
                              <div className="flex-1 min-w-0">
                                <p className="text-sm font-medium truncate" title={filename}>{filename}</p>
                                <p className="text-xs text-muted-foreground truncate opacity-70" title={path}>{path}</p>
                              </div>
                            </div>
                          );
                        })}
                      </div>
                    )}

                    {hasTextBubble && (
                      <div
                        className={`rounded-lg px-4 py-2 w-max max-w-full ${msg.Role === "user"
                          ? "bg-primary text-primary-foreground"
                          : msg.Role === "system"
                            ? "bg-destructive/10 text-destructive text-sm"
                            : "bg-muted"
                          }`}
                      >
                        {msg.Role === "user" ? (
                          <div className="whitespace-pre-wrap break-words">
                            {renderMessageText(text, true)}
                          </div>
                        ) : (
                          <div className="prose prose-sm max-w-none break-words dark:prose-invert">
                            {msg.Thought && (
                              <details className="mb-4 group border rounded-md">
                                <summary className="cursor-pointer font-medium text-xs px-3 py-2 text-muted-foreground hover:bg-muted/50 select-none list-none flex items-center gap-1 transition-colors">
                                  <ChevronRightIcon className="h-3 w-3 transition-transform group-open:rotate-90" />
                                  {t("chat.thoughtProcess", "Thought Process")}
                                </summary>
                                <div className="p-3 pt-2 text-xs text-muted-foreground border-t bg-muted/10 whitespace-pre-wrap max-h-64 overflow-y-auto font-sans leading-relaxed">
                                  {msg.Thought}
                                </div>
                              </details>
                            )}

                            {loading && idx === messages.length - 1 ? (
                              <div className="flex items-start gap-2 text-muted-foreground">
                                {(!text) && (
                                  <Loader2Icon className="h-4 w-4 animate-spin mt-0.5 shrink-0" />
                                )}
                                <div className="flex-1 min-w-0">
                                  {text ? (
                                    renderMessageText(text, false)
                                  ) : (
                                    <span className="italic text-sm">{msg.Thought ? "" : t("chat.thinking", "Thinking...")}</span>
                                  )}
                                </div>
                              </div>
                            ) : (
                              text ? (
                                renderMessageText(text, false)
                              ) : null
                            )}
                          </div>
                        )}
                      </div>
                    )}
                  </div>
                  {msg.Role === "user" && (
                    <div className="flex h-8 w-8 shrink-0 select-none items-center justify-center rounded-md border bg-primary text-primary-foreground shadow-sm">
                      <UserIcon className="h-4 w-4" />
                    </div>
                  )}
                </div>
              )
            })
          )}
          <div ref={messagesEndRef} />
        </div>
        <div className="p-4 border-t bg-background shrink-0 flex flex-col gap-2 relative">
          {loading && (
            <div className="absolute -top-12 left-0 right-0 flex justify-center z-10 w-full">
              <Button
                variant="outline"
                size="sm"
                className="rounded-full shadow-md bg-background hover:bg-muted font-medium text-xs"
                onClick={() => {
                  if (abortController) {
                    abortController.abort()
                    setAbortController(null)
                  }
                }}
              >
                <div className="mr-2 h-2.5 w-2.5 bg-foreground rounded-sm" />
                {t("chat.stop", "Stop generating")}
              </Button>
            </div>
          )}
          <form
            onSubmit={handleSend}
            className="flex w-full items-center space-x-2"
          >
            <Input
              value={input}
              onChange={(e) => setInput(e.target.value)}
              placeholder={t("chat.placeholder", "Type your message to the agent...")}
              className="flex-1"
              disabled={loading || fetching}
            />
            <Button type="submit" size="icon" disabled={!input.trim() || loading || fetching}>
              <SendIcon className="h-4 w-4" />
              <span className="sr-only">Send</span>
            </Button>
          </form>
        </div>
      </div>
      <Dialog open={showClearConfirm} onOpenChange={setShowClearConfirm}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("chat.confirmClearTitle", "Clear Chat Memory")}</DialogTitle>
            <DialogDescription className="text-base text-foreground mt-2">
              {t("chat.confirmClear", "Are you sure you want to clear short-term chat memory?")}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setShowClearConfirm(false)}>{t("common.cancel", "Cancel")}</Button>
            <Button variant="destructive" onClick={handleConfirmClear}>{t("common.confirm", "Confirm")}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
