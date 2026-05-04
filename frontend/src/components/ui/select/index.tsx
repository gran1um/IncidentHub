"use client"

import * as React from "react"
import * as SelectPrimitive from "@radix-ui/react-select"
import { Check, ChevronDown, ChevronUp } from "lucide-react"

import { cn } from "@/lib/utils"

const Select = SelectPrimitive.Root

const SelectGroup = SelectPrimitive.Group

const SelectValue = SelectPrimitive.Value

const SelectTrigger = React.forwardRef<
  React.ElementRef<typeof SelectPrimitive.Trigger>,
  React.ComponentPropsWithoutRef<typeof SelectPrimitive.Trigger>
>(({ className, children, ...props }, ref) => (
  <SelectPrimitive.Trigger
    ref={ref}
    className={cn(
      "flex h-10 w-full items-center justify-between whitespace-nowrap rounded-[10px] border border-[#2a2c3c] bg-[#0b0c10] px-3 py-2 text-sm text-[#f3f4f6] shadow-none ring-offset-background transition-colors data-[placeholder]:text-[#6b7280] focus:outline-none focus:ring-1 focus:ring-[#66ff4c]/55 disabled:cursor-not-allowed disabled:opacity-50 [&>span]:line-clamp-1",
      className
    )}
    {...props}
  >
    {children}
    <SelectPrimitive.Icon asChild>
      <ChevronDown className="h-4 w-4 text-[#6b7280]" />
    </SelectPrimitive.Icon>
  </SelectPrimitive.Trigger>
))
SelectTrigger.displayName = SelectPrimitive.Trigger.displayName

const SelectScrollUpButton = React.forwardRef<
  React.ElementRef<typeof SelectPrimitive.ScrollUpButton>,
  React.ComponentPropsWithoutRef<typeof SelectPrimitive.ScrollUpButton>
>(({ className, ...props }, ref) => (
  <SelectPrimitive.ScrollUpButton
    ref={ref}
    className={cn(
      "flex cursor-default items-center justify-center py-1 text-[#6b7280]",
      className
    )}
    {...props}
  >
    <ChevronUp className="h-4 w-4" />
  </SelectPrimitive.ScrollUpButton>
))
SelectScrollUpButton.displayName = SelectPrimitive.ScrollUpButton.displayName

const SelectScrollDownButton = React.forwardRef<
  React.ElementRef<typeof SelectPrimitive.ScrollDownButton>,
  React.ComponentPropsWithoutRef<typeof SelectPrimitive.ScrollDownButton>
>(({ className, ...props }, ref) => (
  <SelectPrimitive.ScrollDownButton
    ref={ref}
    className={cn(
      "flex cursor-default items-center justify-center py-1 text-[#6b7280]",
      className
    )}
    {...props}
  >
    <ChevronDown className="h-4 w-4" />
  </SelectPrimitive.ScrollDownButton>
))
SelectScrollDownButton.displayName =
  SelectPrimitive.ScrollDownButton.displayName

type SelectContentProps = React.ComponentPropsWithoutRef<typeof SelectPrimitive.Content> & {
  searchable?: boolean
  searchPlaceholder?: string
  emptySearchLabel?: string
  searchThreshold?: number
}

function flattenText(value: React.ReactNode): string {
  if (typeof value === "string" || typeof value === "number") {
    return String(value)
  }
  if (Array.isArray(value)) {
    return value.map(flattenText).join(" ")
  }
  if (React.isValidElement(value)) {
    return flattenText(value.props?.children)
  }
  return ""
}

function countSelectItems(children: React.ReactNode): number {
  return React.Children.toArray(children).reduce<number>((total, child) => {
    if (!React.isValidElement(child)) {
      return total
    }
    if (child.type === SelectItem) {
      return total + 1
    }
    if (child.props?.children) {
      return total + countSelectItems(child.props.children)
    }
    return total
  }, 0)
}

function filterSelectItems(children: React.ReactNode, query: string): React.ReactNode {
  const loweredQuery = query.trim().toLowerCase()
  if (!loweredQuery) {
    return children
  }

  return React.Children.toArray(children).flatMap((child) => {
    if (!React.isValidElement(child)) {
      return []
    }

    if (child.type === SelectItem) {
      const searchText = String(
        child.props?.searchText ?? child.props?.textValue ?? flattenText(child.props?.children)
      )
        .trim()
        .toLowerCase()
      if (searchText.includes(loweredQuery)) {
        return [child]
      }
      return []
    }

    if (child.props?.children) {
      const filteredChildren = filterSelectItems(child.props.children, loweredQuery)
      if (countSelectItems(filteredChildren) === 0) {
        return []
      }
      return [React.cloneElement(child, child.props, filteredChildren)]
    }

    return []
  })
}

const SelectContent = React.forwardRef<
  React.ElementRef<typeof SelectPrimitive.Content>,
  SelectContentProps
>(
  (
    {
      className,
      children,
      position = "popper",
      searchable,
      searchPlaceholder = "Search...",
      emptySearchLabel = "No matching items",
      searchThreshold = 8,
      ...props
    },
    ref
  ) => {
    const [query, setQuery] = React.useState("")
    const itemCount = React.useMemo(() => countSelectItems(children), [children])
    const showSearch = searchable ?? itemCount >= searchThreshold
    const filteredChildren = React.useMemo(
      () => (showSearch ? filterSelectItems(children, query) : children),
      [children, showSearch, query]
    )
    const hasMatches = countSelectItems(filteredChildren) > 0

    return (
      <SelectPrimitive.Portal>
        <SelectPrimitive.Content
          ref={ref}
          className={cn(
            "relative z-50 max-h-[min(24rem,var(--radix-select-content-available-height))] min-w-[8rem] overflow-hidden rounded-[20px] border border-[#4b5563] bg-[#13141c] text-[#f3f4f6] shadow-[0_18px_44px_rgba(0,0,0,0.55)] data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95 data-[side=bottom]:slide-in-from-top-2 data-[side=left]:slide-in-from-right-2 data-[side=right]:slide-in-from-left-2 data-[side=top]:slide-in-from-bottom-2 origin-[--radix-select-content-transform-origin]",
            position === "popper" &&
              "data-[side=bottom]:translate-y-1 data-[side=left]:-translate-x-1 data-[side=right]:translate-x-1 data-[side=top]:-translate-y-1",
            className
          )}
          position={position}
          {...props}
        >
          {showSearch && (
            <div className="border-b border-[#2a2c3c] px-2 py-1.5">
              <input
                autoFocus
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                placeholder={searchPlaceholder}
                className="h-8 w-full rounded-md border border-[#2a2c3c] bg-[#0b0c10] px-2 text-sm text-[#f3f4f6] outline-none placeholder:text-[#6b7280] focus:ring-1 focus:ring-[#66ff4c]/55"
              />
            </div>
          )}
          <SelectScrollUpButton />
          <SelectPrimitive.Viewport
            className={cn(
              "max-h-[18rem] overflow-y-auto p-2",
              position === "popper" &&
                "w-full min-w-[var(--radix-select-trigger-width)]"
            )}
          >
            {hasMatches ? filteredChildren : <div className="px-2 py-2 text-sm text-[#6b7280]">{emptySearchLabel}</div>}
          </SelectPrimitive.Viewport>
          <SelectScrollDownButton />
        </SelectPrimitive.Content>
      </SelectPrimitive.Portal>
    )
  }
)
SelectContent.displayName = SelectPrimitive.Content.displayName

type SelectItemProps = React.ComponentPropsWithoutRef<typeof SelectPrimitive.Item> & {
  searchText?: string
}

const SelectLabel = React.forwardRef<
  React.ElementRef<typeof SelectPrimitive.Label>,
  React.ComponentPropsWithoutRef<typeof SelectPrimitive.Label>
>(({ className, ...props }, ref) => (
  <SelectPrimitive.Label
    ref={ref}
    className={cn("px-2 py-1.5 text-xs font-semibold uppercase tracking-[0.08em] text-[#6b7280]", className)}
    {...props}
  />
))
SelectLabel.displayName = SelectPrimitive.Label.displayName

const SelectItem = React.forwardRef<
  React.ElementRef<typeof SelectPrimitive.Item>,
  SelectItemProps
>(({ className, children, searchText, ...props }, ref) => (
  <SelectPrimitive.Item
    ref={ref}
    className={cn(
      "relative flex w-full cursor-default select-none items-center rounded-[10px] py-2.5 pl-9 pr-3 text-sm text-[#d1d5db] outline-none transition-colors hover:bg-[#1d2436] focus:bg-[#1d2436] focus:text-white data-[state=checked]:bg-[#2367b3] data-[state=checked]:text-white data-[disabled]:pointer-events-none data-[disabled]:opacity-50",
      className
    )}
    data-search-text={searchText || flattenText(children)}
    {...props}
  >
    <span className="absolute left-3 flex h-4 w-4 items-center justify-center">
      <SelectPrimitive.ItemIndicator>
        <Check className="h-4 w-4" />
      </SelectPrimitive.ItemIndicator>
    </span>
    <SelectPrimitive.ItemText>{children}</SelectPrimitive.ItemText>
  </SelectPrimitive.Item>
))
SelectItem.displayName = SelectPrimitive.Item.displayName

const SelectSeparator = React.forwardRef<
  React.ElementRef<typeof SelectPrimitive.Separator>,
  React.ComponentPropsWithoutRef<typeof SelectPrimitive.Separator>
>(({ className, ...props }, ref) => (
  <SelectPrimitive.Separator
    ref={ref}
    className={cn("-mx-1 my-1 h-px bg-[#2a2c3c]", className)}
    {...props}
  />
))
SelectSeparator.displayName = SelectPrimitive.Separator.displayName

export {
  Select,
  SelectGroup,
  SelectValue,
  SelectTrigger,
  SelectContent,
  SelectLabel,
  SelectItem,
  SelectSeparator,
  SelectScrollUpButton,
  SelectScrollDownButton,
}
