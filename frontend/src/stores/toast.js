import { defineStore } from 'pinia'

export const useToastStore = defineStore('toast', {
	state: () => ({
		visible: false,
		title: '',
		message: '',
		type: 'warning', // warning | error | success | info
		_timer: null,
	}),
	actions: {
		show({ title = '提示', message = '', type = 'warning', duration = 3000 } = {}) {
			this.title = title
			this.message = message
			this.type = type
			this.visible = true

			if (this._timer) clearTimeout(this._timer)
			this._timer = setTimeout(() => {
				this.hide()
			}, duration)
		},
		hide() {
			this.visible = false
			if (this._timer) {
				clearTimeout(this._timer)
				this._timer = null
			}
		},
	},
})