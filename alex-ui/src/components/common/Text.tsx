import React from 'react'
import { Text as InkText, TextProps as InkTextProps } from 'ink'

export interface TextProps extends InkTextProps {
  children?: React.ReactNode
}

export const Text: React.FC<TextProps> = ({ children, ...props }) => {
  return <InkText {...props}>{children}</InkText>
}
