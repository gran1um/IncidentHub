import { cn } from "@/lib/utils"

function Skeleton({
  className,
  ...props
}: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn("animate-shimmer bg-gradient-to-r from-primary/5 via-primary/15 to-primary/5 rounded-md", className)}
      {...props}
    />
  )
}

export { Skeleton }
