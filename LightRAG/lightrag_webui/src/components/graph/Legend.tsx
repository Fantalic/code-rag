import React from 'react'
import { useTranslation } from 'react-i18next'
import { useGraphStore } from '@/stores/graph'
import { Card } from '@/components/ui/Card'
import { ScrollArea } from '@/components/ui/ScrollArea'

// Edge keyword color mapping — must match useLightragGraph.tsx
const EDGE_KEYWORD_COLORS: Record<string, string> = {
  calls: '#E74C3C',
  defines: '#2ECC71',
  method: '#27AE60',
  extends: '#9B59B6',
  inheritance: '#8E44AD',
  implements: '#1ABC9C',
  has: '#3498DB',
  property: '#2980B9',
  depends_on: '#F39C12',
  references: '#1ABC9C',
}

interface LegendProps {
  className?: string
}

const Legend: React.FC<LegendProps> = ({ className }) => {
  const { t } = useTranslation()
  const typeColorMap = useGraphStore.use.typeColorMap()
  const hasNodeTypes = typeColorMap && typeColorMap.size > 0

  return (
    <Card className={`p-2 max-w-xs ${className}`}>
      <h3 className="text-sm font-medium mb-2">{t('graphPanel.legend')}</h3>
      <ScrollArea className="max-h-80">
        {hasNodeTypes && (
          <>
            <h4 className="text-xs font-semibold text-muted-foreground mb-1">Entity Types</h4>
            <div className="flex flex-col gap-1 mb-3">
              {Array.from(typeColorMap.entries()).map(([type, color]) => (
                <div key={type} className="flex items-center gap-2">
                  <div
                    className="w-4 h-4 rounded-full"
                    style={{ backgroundColor: color }}
                  />
                  <span className="text-xs truncate" title={type}>
                    {t(`graphPanel.nodeTypes.${type.toLowerCase().replace(/\s+/g, '')}`, type)}
                  </span>
                </div>
              ))}
            </div>
          </>
        )}
        <h4 className="text-xs font-semibold text-muted-foreground mb-1">Relation Types</h4>
        <div className="flex flex-col gap-1">
          {Object.entries(EDGE_KEYWORD_COLORS).map(([keyword, color]) => (
            <div key={keyword} className="flex items-center gap-2">
              <div
                className="w-4 h-1 rounded-sm"
                style={{ backgroundColor: color }}
              />
              <span className="text-xs truncate" title={keyword}>
                {keyword}
              </span>
            </div>
          ))}
        </div>
      </ScrollArea>
    </Card>
  )
}

export default Legend
