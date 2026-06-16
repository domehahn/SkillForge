import { createContext, useContext, useState, useCallback } from 'react'

const ConfirmContext = createContext(null)

export function ConfirmProvider({ children }) {
  const [state, setState] = useState(null)

  const confirm = useCallback((message, { confirmLabel = 'Confirm', danger = false } = {}) => {
    return new Promise(resolve => {
      setState({ message, confirmLabel, danger, resolve })
    })
  }, [])

  const handleClose = (result) => {
    state.resolve(result)
    setState(null)
  }

  return (
    <ConfirmContext.Provider value={confirm}>
      {children}
      {state && (
        <div className="modal-overlay" onClick={() => handleClose(false)}>
          <div className="modal-box" onClick={e => e.stopPropagation()}>
            <p className="modal-message">{state.message}</p>
            <div className="modal-actions">
              <button className="btn-secondary" onClick={() => handleClose(false)}>
                Cancel
              </button>
              <button
                className={`btn-primary${state.danger ? ' btn-danger' : ''}`}
                onClick={() => handleClose(true)}
              >
                {state.confirmLabel}
              </button>
            </div>
          </div>
        </div>
      )}
    </ConfirmContext.Provider>
  )
}

export function useConfirm() {
  return useContext(ConfirmContext)
}
