import React from 'react'
import { Box } from 'ink'
import { useUIStore } from '@/stores'
import { Header } from './Header'
import { Footer } from './Footer'
import { Sidebar } from './Sidebar'

export interface MainLayoutProps {
  children: React.ReactNode
  showHeader?: boolean
  showFooter?: boolean
  showSidebar?: boolean
  sidebarWidth?: number
}

export const MainLayout: React.FC<MainLayoutProps> = ({
  children,
  showHeader = true,
  showFooter = true,
  showSidebar = false, // Default to false for cleaner terminal UI
  sidebarWidth = 30,
}) => {
  const { headerVisible, sidebarOpen } = useUIStore()

  const shouldShowHeader = showHeader && headerVisible
  const shouldShowSidebar = showSidebar && sidebarOpen

  return (
    <Box flexDirection="column" height="100%">
      {/* Header */}
      {shouldShowHeader && (
        <Box>
          <Header />
        </Box>
      )}

      {/* Main content area */}
      <Box flexGrow={1} flexDirection="row">
        {/* Sidebar */}
        {shouldShowSidebar && (
          <Box>
            <Sidebar width={sidebarWidth} />
          </Box>
        )}

        {/* Main content */}
        <Box flexGrow={1} flexDirection="column">
          {children}
        </Box>
      </Box>

      {/* Footer */}
      {showFooter && (
        <Box>
          <Footer />
        </Box>
      )}
    </Box>
  )
}