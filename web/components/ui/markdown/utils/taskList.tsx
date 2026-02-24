import { Children, ReactNode, isValidElement } from "react";

export function isTaskListClass(className: string | undefined): boolean {
  return typeof className === "string" && className.includes("contains-task-list");
}

export function splitTaskListChildren(children: ReactNode) {
  const nodes = Children.toArray(children);
  const checkboxIndex = nodes.findIndex(
    (child) =>
      isValidElement(child) &&
      child.type === "input" &&
      (child.props as { type?: string }).type === "checkbox",
  );
  if (checkboxIndex === -1) {
    return null;
  }
  const checkbox = nodes[checkboxIndex];
  const rest = nodes.filter((_, index) => index !== checkboxIndex);
  return { checkbox, rest };
}

export function renderLineBreaks(children: ReactNode) {
  const nodes = Children.toArray(children);
  const output: ReactNode[] = [];
  nodes.forEach((child, childIndex) => {
    if (typeof child !== "string") {
      output.push(child);
      return;
    }
    const parts = child.split("\n");
    parts.forEach((part, partIndex) => {
      if (partIndex > 0) {
        output.push(<br key={`br-${childIndex}-${partIndex}`} />);
      }
      if (part !== "") {
        output.push(part);
      }
    });
  });
  return output;
}
