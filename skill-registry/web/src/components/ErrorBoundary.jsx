import { Component } from 'react'

export default class ErrorBoundary extends Component {
  constructor(props) {
    super(props)
    this.state = { error: null }
  }

  static getDerivedStateFromError(error) {
    return { error }
  }

  render() {
    if (this.state.error) {
      return (
        <div className="error-boundary">
          <div className="error-boundary-code">⚠</div>
          <h1 className="error-boundary-title">Something went wrong</h1>
          <p className="error-boundary-msg">
            {this.state.error?.message || 'An unexpected error occurred.'}
          </p>
          <button
            className="btn-primary"
            onClick={() => {
              this.setState({ error: null })
              window.location.href = '/'
            }}
          >
            Go to home
          </button>
        </div>
      )
    }
    return this.props.children
  }
}
