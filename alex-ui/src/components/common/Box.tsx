import React from 'react'
import { Box as InkBox, BoxProps as InkBoxProps } from 'ink'

export interface BoxProps extends InkBoxProps {
  children?: React.ReactNode
}

export const Box: React.FC<BoxProps> = ({ children, ...props }) => {
  return <InkBox {...props}>{children}</InkBox>
}