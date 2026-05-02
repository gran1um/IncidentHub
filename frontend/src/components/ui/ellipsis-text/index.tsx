import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip"
import { cn } from "@/lib/utils"
import { useCallback, useEffect, useRef, useState, type HTMLAttributes } from "react"

type EllipsisTextProps = HTMLAttributes<HTMLSpanElement> & {
  text: string | number | null | undefined
  tooltipClassName?: string
}

export function EllipsisText({ text, className, tooltipClassName, ...props }: EllipsisTextProps) {
  const value = String(text ?? "").trim()
  const { onMouseEnter, onFocus, ...restProps } = props
  const textRef = useRef<HTMLSpanElement>(null)
  const [isTruncated, setIsTruncated] = useState(false)

  const evaluateTruncation = useCallback(() => {
    const node = textRef.current
    if (!node) {
      setIsTruncated(false)
      return
    }
    const computed = window.getComputedStyle(node)
    const lineClampValue = Number.parseInt(computed.getPropertyValue("-webkit-line-clamp"), 10)
    const hasLineClamp = Number.isFinite(lineClampValue) && lineClampValue > 0
    const hasSingleLineEllipsis = computed.textOverflow === "ellipsis" && computed.whiteSpace.includes("nowrap")

    if (!hasLineClamp && !hasSingleLineEllipsis) {
      setIsTruncated(false)
      return
    }

    const horizontalOverflow = node.scrollWidth - node.clientWidth > 2
    const verticalOverflow = node.scrollHeight - node.clientHeight > 2
    setIsTruncated(horizontalOverflow || verticalOverflow)
  }, [])

  useEffect(() => {
    evaluateTruncation()
  }, [evaluateTruncation, value, className])

  useEffect(() => {
    const node = textRef.current
    if (!node) {
      return
    }

    evaluateTruncation()

    if (typeof ResizeObserver !== "undefined") {
      const observer = new ResizeObserver(() => evaluateTruncation())
      observer.observe(node)
      return () => observer.disconnect()
    }

    const onResize = () => evaluateTruncation()
    window.addEventListener("resize", onResize)
    return () => window.removeEventListener("resize", onResize)
  }, [evaluateTruncation, value])

  useEffect(() => {
    if (typeof document === "undefined") {
      return
    }
    const fontSet = document.fonts
    if (!fontSet) {
      return
    }

    let cancelled = false
    fontSet.ready
      .then(() => {
        if (!cancelled) {
          evaluateTruncation()
        }
      })
      .catch(() => undefined)

    const onFontsLoaded = () => evaluateTruncation()
    fontSet.addEventListener("loadingdone", onFontsLoaded)
    return () => {
      cancelled = true
      fontSet.removeEventListener("loadingdone", onFontsLoaded)
    }
  }, [evaluateTruncation])

  const content = (
    <span
      ref={textRef}
      className={cn("block min-w-0 overflow-hidden text-ellipsis whitespace-nowrap", className)}
      onMouseEnter={(event) => {
        evaluateTruncation()
        onMouseEnter?.(event)
      }}
      onFocus={(event) => {
        evaluateTruncation()
        onFocus?.(event)
      }}
      {...restProps}
    >
      {value}
    </span>
  )

  if (!value || !isTruncated) {
    return content
  }

  return (
    <TooltipProvider delayDuration={150}>
      <Tooltip>
        <TooltipTrigger asChild>{content}</TooltipTrigger>
        <TooltipContent
          side="top"
          align="start"
          className={cn("max-w-[min(36rem,calc(100vw-2rem))] whitespace-normal break-words", tooltipClassName)}
        >
          {value}
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  )
}
